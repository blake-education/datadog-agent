# Agent Configuration Schema

The Datadog Agent configuration schema is a YAML-based
[JSON Schema](https://json-schema.org/) that centrally describes every
configuration setting for the Agent. It is written in YAML for readability but
is fully compatible with JSON Schema tooling.

## Why it exists

Previously, Agent configuration was defined through a combination of imperative Go code (`BindEnv` and
`BindEnvAndSetDefault` calls in `pkg/config/setup`), YAML example files manually maintained, and scattered validation
logic. This made it hard to understand what a setting does, what values it accepts, or what its default is — without
reading source code.

The schema replaces this with a **single source of truth**. All information
about a setting — its type, default value, documentation, environment variables,
validation rules, and visibility — lives in one place.

## What it enables

- **Config validation without running the Agent** — any JSON Schema library can
  validate a customer's `datadog.yaml` against the schema.
- **IDE autocompletion** — the schema can be published to
  [SchemaStore](https://www.schemastore.org/) so editors like VS Code
  automatically validate and complete configuration files.
- **Automatic generation of `datadog.yaml.example` and `system-probe.yaml.example** — the example file shipped with the
  Agent is generated directly from the schema, ensuring it stays in sync.
- **Type-safe code generation** — Go configuration code can be generated from
  the schema, removing the imperative setup code entirely.

## Node types

The schema tree is composed of two types of nodes:

- **Leaf nodes** represent individual settings. They have a type and a value.
  For example, `apm_config.enabled` is a leaf node of type `boolean`.
- **Section nodes** represent groups of settings. Sections do not have values
  themselves; they group related leaf nodes together. For example, `apm_config`
  is a section node containing `enabled` and many other leaf nodes. Section
  nodes are identified by the `node_type: section` keyword.

The distinction matters when a setting's value is itself a map or object. For
example, `docker_labels_as_tags` is a `type: object` leaf node — its value is a
dict of strings, but that dict is the setting's *value*, not a group of child
settings. It is **not** a section node.

## One schema per configuration file

The Agent ships with multiple configuration files, each with its own schema:

| Config file | Schema file |
| --- | --- |
| `datadog.yaml` | `pkg/config/schema/core_schema_enriched.yaml` |
| `system-probe.yaml` | `pkg/config/schema/system-probe_schema_enriched.yaml` |

All schemas share the same keyword set described in this documentation.

## JSON Schema foundation

The Agent schema builds on [JSON Schema draft 2020-12](https://json-schema.org/).
This documentation focuses on how keywords are used in the Agent schema
specifically. For a general introduction to JSON Schema, see
[Understanding JSON Schema](https://json-schema.org/understanding-json-schema).

## Next steps

- [Keyword Reference](keywords.md) — complete reference for all supported keywords.
- [Examples](examples.md) — annotated, real-world examples from the schema.
- [FAQ](faq.md) — common tasks such as adding a setting or making one public.
