#ifndef _HELPERS_SPAN_H_
#define _HELPERS_SPAN_H_

#include "maps.h"

#include "process.h"

// --- Datadog proprietary span TLS (existing mechanism) ---

int __attribute__((always_inline)) handle_register_span_memory(void *data) {
    struct span_tls_t tls = {};
    bpf_probe_read(&tls, sizeof(tls), data);

    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = pid_tgid >> 32;

    bpf_map_update_elem(&span_tls, &tgid, &tls, BPF_NOEXIST);

    return 0;
}

int __attribute__((always_inline)) unregister_span_memory() {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = pid_tgid >> 32;

    bpf_map_delete_elem(&span_tls, &tgid);

    return 0;
}

// --- OTel Thread Local Context Record (per OTel spec PR #4947) ---
// Targets native applications using ELF TLSDESC (C, C++, Rust, Java/JNI, etc.).
// Support for additional runtimes (e.g., Go via pprof labels) will be added later.

int __attribute__((always_inline)) handle_register_otel_tls(void *data) {
    struct otel_tls_t tls = {};
    bpf_probe_read(&tls, sizeof(tls), data);

    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = pid_tgid >> 32;

    bpf_map_update_elem(&otel_tls, &tgid, &tls, BPF_NOEXIST);

    return 0;
}

int __attribute__((always_inline)) unregister_otel_tls() {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = pid_tgid >> 32;

    bpf_map_delete_elem(&otel_tls, &tgid);

    return 0;
}

// Convert 8 bytes in W3C (big-endian / network byte order) to a native-endian u64.
static u64 __attribute__((always_inline)) otel_bytes_to_u64(const u8 *bytes) {
    return ((u64)bytes[0] << 56) | ((u64)bytes[1] << 48) |
           ((u64)bytes[2] << 40) | ((u64)bytes[3] << 32) |
           ((u64)bytes[4] << 24) | ((u64)bytes[5] << 16) |
           ((u64)bytes[6] << 8)  | ((u64)bytes[7]);
}

// Try to fill span context from an OTel Thread Local Context Record.
// Returns 1 on success, 0 otherwise.
// Architecture: x86_64 only (reads fsbase from task_struct->thread.fsbase).
// ARM64 support (tpidr_el0) will be added later.
static int __attribute__((always_inline)) fill_span_context_otel(struct span_context_t *span) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = pid_tgid >> 32;

    struct otel_tls_t *otls = bpf_map_lookup_elem(&otel_tls, &tgid);
    if (!otls) {
        return 0;
    }

    // Read fsbase (thread pointer) from task_struct->thread.fsbase.
    // The two offsets are summed because "thread" is a named (non-anonymous)
    // member of task_struct, so we need separate BTF lookups.
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u64 thread_offset = get_task_struct_thread_offset();
    u64 fsbase_offset = get_thread_struct_fsbase_offset();

    u64 fsbase = 0;
    int ret = bpf_probe_read_kernel(&fsbase, sizeof(fsbase),
                                     (void *)task + thread_offset + fsbase_offset);
    if (ret < 0 || fsbase == 0) {
        return 0;
    }

    // The TLSDESC TLS variable is a pointer to the active Thread Local Context Record.
    // Read the pointer at [fsbase + tls_offset].
    void *record_ptr = NULL;
    ret = bpf_probe_read_user(&record_ptr, sizeof(record_ptr),
                               (void *)(fsbase + otls->tls_offset));
    if (ret < 0 || record_ptr == NULL) {
        return 0;
    }

    // Read the OTel Thread Local Context Record (28-byte fixed header).
    struct otel_thread_ctx_record_t record = {};
    ret = bpf_probe_read_user(&record, sizeof(record), record_ptr);
    if (ret < 0) {
        return 0;
    }

    // The record is only valid when the valid field is exactly 1.
    if (record.valid != 1) {
        return 0;
    }

    // Convert W3C byte order (big-endian) to native-endian span_context_t.
    // OTel trace-id: bytes[0..7] = high 64 bits, bytes[8..15] = low 64 bits.
    span->trace_id[1] = otel_bytes_to_u64(&record.trace_id[0]);  // Hi
    span->trace_id[0] = otel_bytes_to_u64(&record.trace_id[8]);  // Lo
    span->span_id = otel_bytes_to_u64(record.span_id);

    return 1;
}

// --- Unified span context fill ---

void __attribute__((always_inline)) fill_span_context(struct span_context_t *span) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = pid_tgid >> 32;

    // Try Datadog proprietary TLS first (existing behavior).
    struct span_tls_t *tls = bpf_map_lookup_elem(&span_tls, &tgid);
    if (tls) {
        u32 tid = pid_tgid;

        struct task_struct *current_ptr = (struct task_struct *)bpf_get_current_task();
        u32 pid = get_namespace_nr_from_task_struct(current_ptr);
        if (pid) {
            tid = pid;
        }

        int offset = (tid % tls->max_threads) * sizeof(struct span_context_t);
        int ret = bpf_probe_read_user(span, sizeof(struct span_context_t), tls->base + offset);
        if (ret >= 0 && (span->span_id != 0 || span->trace_id[0] != 0 || span->trace_id[1] != 0)) {
            return;
        }
    }

    // Fall back to OTel Thread Local Context Record (native applications only).
    if (fill_span_context_otel(span)) {
        return;
    }

    // No span context available.
    span->span_id = 0;
    span->trace_id[0] = span->trace_id[1] = 0;
}

void __attribute__((always_inline)) reset_span_context(struct span_context_t *span) {
    span->span_id = 0;
    span->trace_id[0] = 0;
    span->trace_id[1] = 0;
}

void __attribute__((always_inline)) copy_span_context(struct span_context_t *src, struct span_context_t *dst) {
    dst->span_id = src->span_id;
    dst->trace_id[0] = src->trace_id[0];
    dst->trace_id[1] = src->trace_id[1];
}

#endif
