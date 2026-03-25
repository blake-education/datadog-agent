# Go Generics: Shapes, Dictionaries, and DWARF

This document describes how the Go compiler implements generics at the binary
level, and what that means for dynamic instrumentation. Understanding this is
essential for probing generic functions and correctly interpreting their
parameters.

## Shape Stenciling

The Go compiler does not fully monomorphize generic functions. Instead, it uses
**GC shape stenciling**: type arguments that have the same garbage collection
shape share a single compiled function body. The function operates on the shape
type and uses a runtime **dictionary** to recover concrete type information when
needed (e.g., for method calls, type assertions, reflection).

For a generic function:

```go
func Contains[T comparable](haystack []T, needle T) bool { ... }
```

Called with `int` and `string`, the compiler emits:

| Symbol | What it is |
|--------|-----------|
| `pkg.Contains[go.shape.int]` | Shape function for int-shaped types |
| `pkg.Contains[go.shape.string]` | Shape function for string-shaped types |
| `pkg.Contains[int]` | Trampoline: tail-calls shape function with `&.dict.Contains[int]` |
| `pkg.Contains[string]` | Trampoline: tail-calls shape function with `&.dict.Contains[string]` |
| `pkg..dict.Contains[int]` | Dictionary for the `int` instantiation |
| `pkg..dict.Contains[string]` | Dictionary for the `string` instantiation |

The shape functions contain the real code. The trampolines are thin wrappers
marked with `DW_AT_trampoline` in DWARF.

## What Determines a Shape

Two rules control which types share a shape:

1. **Underlying type identity.** Types with the same `Underlying()` share a
   shape. So `int` and `type MyInt int` both use `go.shape.int`. But `int` and
   `int64` do NOT share a shape — they have different underlying types, even on
   platforms where they're the same size.

2. **Pointer collapsing for basic interfaces.** When a type parameter is
   constrained by a basic interface (one with no methods — like `any` or
   `comparable`), all pointer type arguments collapse to `go.shape.*uint8`.
   So `*Foo`, `*Bar`, and `*string` all share the same shape function. This
   is sound because basic interfaces don't permit accessing the pointee's
   structure. When the constraint has methods, pointers keep their distinct
   shapes.

Consequence: a single shape function can serve **multiple concrete types**. The
dictionary is the runtime discriminator — it tells the function which concrete
type it's operating on.

## The Dictionary

The dictionary is a flat read-only array of `uintptr`-sized words. It has four
sections laid out sequentially:

```
Offset (words)          Section
──────────────          ─────────────────────────────────
0                       typeParamMethodExprs
                        Function pointers for methods called on type parameters.
                        When generic code calls a method on T, it reads the
                        concrete method address from here.

+len(methodExprs)       subdicts
                        Pointers to sub-dictionaries for nested generic calls.
                        If F[T] calls G[T], F's dictionary has a pointer to
                        G's dictionary instantiated with the same types.

+len(subdicts)          rtypes
                        *runtime._type pointers for derived types.
                        Used for reflection, type assertions, and conversions.

+len(rtypes)            itabs
                        *runtime.itab pointers for converting type parameters
                        to non-empty interfaces.
```

Dictionary symbols are named `pkg..dict.Func[ConcreteType]` and placed in
RODATA with DUPOK (linker deduplication).

### Dictionary parameter location

For **out-of-line shape functions**, the dictionary is passed as a hidden
parameter:

- **Generic functions:** the dictionary is the first regular parameter
  (before all user-visible parameters).
- **Methods on generic types:** the dictionary is the first regular parameter
  after the receiver.

The parameter is named `.dict` and typed as `*[N]uintptr`. It follows the
standard Go register-based calling convention (ABIInternal) — there is no
special register reserved for it.

For **closures inside generic functions**, the dictionary is NOT a parameter.
Instead, it is captured as the **last variable** in the closure struct. The
closure shares its parent's dictionary — no separate dictionary is generated
for it.

### Dictionary parameter in DWARF

As of Go 1.23–1.26, the `.dict` parameter is **not emitted to DWARF**. The
compiler adds it at the IR level but strips it from debug info before DWARF
generation. Delve (the Go debugger) knows about this and infers the
dictionary's location from its fixed position in the parameter list.

The name `.dict` is a convention shared between the compiler and Delve:
https://github.com/go-delve/delve/blob/cb91509630529e6055be845688fd21eb89ae8714/pkg/proc/eval.go#L28

## DWARF Representation

### Shape function entries

Shape functions are normal `DW_TAG_subprogram` entries (not trampolines).
Their children include:

- **`DW_TAG_typedef` entries** named `.param0`, `.param1`, etc. Each typedef
  has:
  - `DW_AT_type`: reference to the shape type (e.g., `go.shape.int`)
  - `DW_AT_go_dict_index` (0x2906): the index into the dictionary array
    where the concrete `*runtime._type` for this type parameter lives

- **`DW_TAG_formal_parameter` entries** for the user-visible parameters.
  Their types reference the typedef names (`.param0`, `.param1`), not the
  shape types directly.

The `.dict` parameter itself does **not** appear as a formal parameter in DWARF.

### Trampoline entries

Concrete-type wrappers (e.g., `pkg.Contains[int]`) are marked with
`DW_AT_trampoline` and have the concrete parameter types. They are typically
skipped during instrumentation since they just tail-call the shape function.

### Example

For `main.genericContains[go.shape.int]`:

```
DW_TAG_subprogram "main.genericContains[go.shape.int]"
  DW_TAG_typedef ".param0"
    DW_AT_type → []go.shape.int
    DW_AT_go_dict_index → 0
  DW_TAG_typedef ".param1"
    DW_AT_type → go.shape.int
    DW_AT_go_dict_index → 1
  DW_TAG_formal_parameter "haystack"
    DW_AT_type → .param0
  DW_TAG_formal_parameter "needle"
    DW_AT_type → .param1
  DW_TAG_formal_parameter "~r0"
    DW_AT_type → bool
```

## Implications for Dynamic Instrumentation

### Probing

Shape functions are the correct probe targets — they contain the real code.
Trampolines should be skipped (they're already filtered by the
`DW_AT_trampoline` check).

A single probe on a shape function fires for ALL concrete types that share
that shape. For example, probing `genericContains[go.shape.int]` fires for
calls with `int`, `myInt`, or any other type whose underlying type is `int`.

### Type resolution at probe time

When a probe fires on a shape function, the captured parameter types are shape
types (e.g., `go.shape.int`), not the caller's concrete types. To resolve the
actual concrete type:

1. Read the dictionary pointer from the first parameter register (known
   position in the ABI — first arg for functions, first arg after receiver for
   methods).
2. Index into the dictionary using the `DW_AT_go_dict_index` from the
   parameter's typedef.
3. The dictionary entry is a `*runtime._type` pointer.
4. Look up the concrete type using the existing `typesByGoRuntimeType` mapping.

This is architecturally identical to interface type resolution, where the
runtime type is read from the interface value's type word.

For closures inside generic functions, the dictionary is in the closure struct
(last captured variable) rather than a parameter, requiring an additional
pointer dereference.

### Limitations

- The `.dict` parameter is absent from DWARF, so its location must be inferred
  from ABI knowledge (first param position).
- The dictionary layout (number of entries per section) is not encoded in DWARF.
  The `DW_AT_go_dict_index` values are flat indices into the full array, so
  they can be used directly without knowing section boundaries.
- Multiple concrete types can map to the same shape. Without reading the
  dictionary at runtime, it's impossible to distinguish between them statically.

## References

- [GC Shape Stenciling design doc](https://github.com/golang/proposal/blob/master/design/generics-implementation-gcshape.md)
- [Dictionaries design doc](https://github.com/golang/proposal/blob/master/design/generics-implementation-dictionaries.md)
- [Go 1.18 implementation details](https://github.com/golang/proposal/blob/master/design/generics-implementation-dictionaries-go1.18.md)
- Compiler source: `cmd/compile/internal/noder/reader.go` — `shapeSig`,
  `shapify`, `dictNameOf`, `funcLit`
- DWARF constant: `DW_AT_go_dict_index = 0x2906`
  (`cmd/compile/internal/irgen/dwarf_constants.go` in this repo)
