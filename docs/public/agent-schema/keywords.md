# Keyword Reference

This page documents every keyword supported by the Agent configuration schema.
Keywords are grouped into two sections: standard JSON Schema keywords and
Datadog Agent extensions.

---

## Standard JSON Schema keywords

The following keywords come from the [JSON Schema standard](https://json-schema.org/understanding-json-schema/keywords)
and are understood by all JSON Schema tooling. This section describes how each
keyword is used **specifically in the Agent schema**.

### `type`

The data type of a leaf node's value.

- **Mandatory:** yes, for all leaf nodes. Not valid on section nodes.

Supported types:

| Type | Description |
| --- | --- |
| `boolean` | `true` or `false` |
| `number` | integer or floating-point number |
| `string` | text value |
| `array` | ordered list of values |
| `object` | key/value map (also called a dict) |

Complex types are composed by combining these primitives — for example, an
`array` whose `items` are `object`s.

```yaml
network_devices:
  node_type: leaf
  type: array
  default: []
  items:       # describes the value — each element must be an object
    type: object
```

---

### `default`

The default value for a leaf node.

- **Mandatory:** yes, for all leaf nodes unless `platform_default` is set. The two keywords are mutually exclusive.

The value must match the `type` of the setting.

```yaml
check_runners:
  node_type: leaf
  type: number
  default: 4
```

---

### `description`

Human-readable documentation for a node.

- **Mandatory:** yes, for any node with `visibility: public`. Optional otherwise, but every setting should eventually have one — a developer should not need to read source code to understand what a setting does.
- **Available on:** leaf and section nodes.

Used to generate the `datadog.yaml.example` file shipped with the Agent.

Use the YAML `|` block scalar for multi-line descriptions:

```yaml
api_key:
  node_type: leaf
  type: string
  default: ""
  description: |
    The Datadog API key used by your Agent to submit metrics and events
    to Datadog.

    Create a new API key here: https://app.datadoghq.com/organization-settings/api-keys
```

---

### `title`

A short heading used to generate section banners in `datadog.yaml.example`.

- **Available on:** section nodes only.

When a section has a `title`, the generator produces a banner like this:

```
####################################
## Trace Collection Configuration ##
####################################
```

Full section node example:

```yaml
apm_config:
  node_type: section
  title: "Trace Collection Configuration"
  description: |
    Enter specific configurations for your trace collection.
    Uncomment this parameter and the one below to enable them.
    See https://docs.datadoghq.com/agent/apm/
  visibility: public
  properties:
    enabled:
      node_type: leaf
      type: boolean
      default: false
      description: Enable the APM agent.
```

---

### Validation keywords

Each JSON Schema type comes with built-in validation keywords. The most useful
ones in the Agent schema are listed below. Validation rules can be arbitrarily
nested — see [Examples](examples.md#example-3-complex-nested-type-cel_workload_exclude)
for a real-world demonstration.

| Keyword | Applies to | Description |
| --- | --- | --- |
| `minimum` / `maximum` | `number` | Numeric lower and upper bounds |
| `enum` | `string`, `number`, array items | Restricts the value to a fixed set |
| `pattern` | `string` | Regular expression the value must match |
| `minLength` / `maxLength` | `string` | String length bounds |
| `items` | `array` | Schema that every element of the array must satisfy |
| `properties` | `object` | Named sub-schemas for specific keys |
| `additionalProperties` | `object` | Schema for keys not listed in `properties`, or `false` to forbid unknown keys |
| `required` | `object` | List of keys that must be present |
| `minItems` / `maxItems` | `array` | Array length bounds |
| `uniqueItems` | `array` | Requires all elements to be distinct |

For the complete specification of these keywords, see
[Understanding JSON Schema](https://json-schema.org/understanding-json-schema).

---

## Datadog Agent extensions

The following keywords extend JSON Schema to meet the Agent's specific needs.
They are **not** used for config validation — any standard JSON Schema library
can validate a customer config without them. They are used by the Agent itself
and by Datadog internal tooling.

---

### `node_type`

Declares whether a node is a *section* (group of settings) or a *leaf*
(individual setting).

- **Available on:** all nodes.
- **Mandatory:** yes.
- **Accepted values:** `section`, `leaf`.

This keyword marks the boundary between the schema structure and a setting's
value. For example, `docker_labels_as_tags` has `type: object` — its value is a
dict of strings. That dict is the setting's *value*, so the node is a leaf, not
a section. A node is a section only when its `properties` represent *child
settings*, not the contents of a value.

```yaml
apm_config:
  node_type: section
  title: "Trace Collection Configuration"
  properties:
    enabled:
      node_type: leaf
      type: boolean
      default: false

docker_labels_as_tags:
  node_type: leaf     # value is a dict, but the dict IS the setting's value
  type: object
  default: {}
  additionalProperties:  # describes the value — all values in the dict must be strings
    type: string
```

---

### `platform_default`

Sets different default values per OS or platform. Mutually exclusive with
`default`.

- **Available on:** leaf nodes only.
- **Mandatory:** yes, for leaf nodes that have platform-specific defaults (instead of `default`).
- **Validation:** values must match the `type` of the setting.

Supported platform keys: `linux`, `windows`, `darwin`, `container`.

**Validation:** a `platform_default` entry must provide a default for every
platform. This means either listing `linux`, `windows`, and `darwin`
explicitly, or using `other` as a catch-all fallback.

**Container fallback logic:** because container environments share many
defaults with Linux, `container` is optional. When resolving the default for a
container, the Agent applies the following fallback chain:

1. Use `container` if present.
2. Fall back to `linux` if present.
3. Fall back to `other` if present.

```yaml
# Explicit entry for every platform:
confd_path:
  node_type: leaf
  type: string
  platform_default:
    windows: "C:\\ProgramData\\Datadog\\conf.d"
    linux: "/etc/datadog-agent/conf.d"
    darwin: "/opt/datadog-agent/etc/conf.d"
    container: "/conf.d"    # optional — falls back to linux if omitted

# Using 'other' as a catch-all:
gui_port:
  node_type: leaf
  type: number
  platform_default:
    linux: -1
    other: 5002             # covers windows and darwin
```

---

### `visibility`

Controls whether a setting is publicly documented and included in
`datadog.yaml.example`.

- **Available on:** all nodes (leaf and section).
- **Mandatory:** no.
- **Default:** `undocumented`.
- **Accepted values:** `public`, `undocumented`.

Any node with `visibility: public` is included in the generated config templates
and any public-facing configuration website. Nodes without this keyword (or with
`visibility: undocumented`) are internal and will not be surfaced.

```yaml
api_key:
  node_type: leaf
  type: string
  default: ""
  description: "Your Datadog API key."
  visibility: public
```

---

### `env_vars`

The list of environment variables that can override this leaf node's value.

- **Available on:** leaf nodes only.
- **Mandatory:** no.

If omitted, the Agent uses a default env var derived from the setting's full
dotted path: `DD_` + the path in upper case with dots replaced by underscores.
For example, `logs_config.enabled` defaults to `DD_LOGS_CONFIG_ENABLED`.

When multiple env vars are listed, they are checked in order and the first match
wins.

```yaml
api_key:
  node_type: leaf
  type: string
  default: ""
  env_vars:
    - DD_API_KEY
    - DATADOG_API_KEY
```

---

### `env_parser`

Defines how the env var value is parsed into the setting's type.

- **Available on:** leaf nodes only.
- **Mandatory:** no. Most scalar types (`boolean`, `number`, `string`) are parsed automatically.

| Value | Behaviour |
| --- | --- |
| `list_string_comma_separated` | Splits on commas |
| `list_string_space_separated` | Splits on spaces |
| `list_string_comma_or_space_separated` | Splits on commas if the value contains a comma, otherwise on spaces. Used by APM for `apm_config.ignore_resources`. |
| `list_string_comma_and_space_separated` | Both commas and spaces act as separators. Used by OTEL for `otelcollector.converter.features`. |
| `list_string_space_or_json` | Splits on spaces, or parses as JSON if the value starts with `[` |
| `json` | Parses the entire env var value as a JSON payload matching the setting's type |

```yaml
tags:
  node_type: leaf
  type: array
  items:        # describes the value — each element must be a string
    type: string
  default: []
  env_vars:
    - DD_TAGS
  env_parser: list_string_space_or_json
```

---

### `renamed_from`

Lists previous names this setting was known by.

- **Available on:** leaf nodes only.
- **Mandatory:** no.
- **Validation:** must be a list of strings.

When a setting has `renamed_from`, the config system looks for any of the
previous names and migrates the value automatically. Previous names take
priority over the canonical name when both are present. A deprecation warning
is emitted at runtime whenever a previous name is used.

This provides a single, consistent mechanism for setting renames across all
teams, replacing the ad-hoc solutions that previously produced inconsistent
behaviour.

```yaml
target_traces_per_second:
  node_type: leaf
  type: number
  default: 10
  renamed_from:
    - max_traces_per_second
```

---

### `tags`

An arbitrary list of strings for metadata. Used by internal tooling to slice
and filter settings — for example, to produce different variants of
`datadog.yaml.example` or to control the order of sections in the generated
file.

- **Available on:** all nodes (leaf and section).
- **Mandatory:** no.
- **Validation:** must be a list of strings.

```yaml
internal_profiling:
  node_type: section
  tags:
    - internal_template_section:profiling
```

Real-world usage: the `template_section` and `template_section_order` tags in
`pkg/config/schema/core_schema_enriched.yaml` control which section each setting belongs
to and its position in the generated `datadog.yaml.example`.
