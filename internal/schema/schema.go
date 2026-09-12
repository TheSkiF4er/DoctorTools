package schema

const ReportJSONSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://gitflic.ru/skif4er/doctortools/schemas/report.schema.json",
  "title": "DoctorTools Report",
  "description": "Стабильная JSON-схема отчётов DoctorTools 2.x и CraftDoctor 2.x.",
  "type": "object",
  "required": ["tool_version", "generated_at", "target_path", "status", "summary", "system", "java", "minecraft", "rules", "findings"],
  "properties": {
    "tool_version": {"type": "string"},
    "generated_at": {"type": "string", "format": "date-time"},
    "target_path": {"type": "string"},
    "status": {"$ref": "#/$defs/severity"},
    "summary": {
      "type": "object",
      "required": ["critical", "danger", "warn", "info"],
      "properties": {
        "critical": {"type": "integer", "minimum": 0},
        "danger": {"type": "integer", "minimum": 0},
        "warn": {"type": "integer", "minimum": 0},
        "info": {"type": "integer", "minimum": 0}
      },
      "additionalProperties": true
    },
    "system": {"type": "object", "additionalProperties": true},
    "java": {"type": "object", "additionalProperties": true},
    "minecraft": {"type": "object", "additionalProperties": true},
    "config": {"type": "object", "additionalProperties": true},
    "plugins": {"type": "array", "items": {"type": "object", "additionalProperties": true}},
    "plugin_audit": {"type": "object", "additionalProperties": true},
    "logs": {"type": "object", "additionalProperties": true},
    "performance": {"type": "object", "additionalProperties": true},
    "proxy": {"type": "object", "additionalProperties": true},
    "security": {
      "type": "object",
      "properties": {
        "status": {"$ref": "#/$defs/severity"},
        "score": {"type": "integer", "minimum": 0, "maximum": 100},
        "checks": {"type": "array"},
        "secrets": {"type": "array"},
        "secrets_ignore_file": {"type": "string"},
        "secrets_ignore_patterns": {"type": "array", "items": {"type": "string"}},
        "secret_files_scanned": {"type": "integer", "minimum": 0},
        "sensitive_files": {"type": "array"},
        "plugin_signals": {"type": "array"},
        "recommendations": {"type": "array", "items": {"type": "string"}}
      },
      "additionalProperties": true
    },
    "production": {"type": "object", "additionalProperties": true},
    "rules": {
      "type": "object",
      "properties": {
        "catalog_version": {"type": "string"},
        "builtin_rules": {"type": "integer", "minimum": 0},
        "custom_rules_file": {"type": "string"},
        "ignore_file": {"type": "string"},
        "ignore_patterns": {"type": "array", "items": {"type": "string"}},
        "suppressed_findings": {"type": "integer", "minimum": 0}
      },
      "additionalProperties": true
    },
    "findings": {"type": "array", "items": {"$ref": "#/$defs/finding"}},
    "suppressed_findings": {"type": "array", "items": {"$ref": "#/$defs/finding"}}
  },
  "additionalProperties": true,
  "$defs": {
    "severity": {"type": "string", "enum": ["INFO", "WARN", "DANGER", "CRITICAL"]},
    "finding": {
      "type": "object",
      "required": ["id", "severity", "category", "title", "message"],
      "properties": {
        "id": {"type": "string"},
        "severity": {"$ref": "#/$defs/severity"},
        "category": {"type": "string"},
        "title": {"type": "string"},
        "message": {"type": "string"},
        "recommendation": {"type": "string"},
        "file": {"type": "string"}
      },
      "additionalProperties": true
    }
  }
}
`
