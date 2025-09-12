package utils

import (
	opencncModel "OpenCNC_config_service/opencnc_model"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// ResolveNamespace splits a "prefix:tag" and finds the namespace URI.
func ResolveNamespace(prefixedKey string) (namespaceURI, tag string, found bool) {
	parts := strings.SplitN(prefixedKey, ":", 2)
	if len(parts) != 2 {
		return "", prefixedKey, false
	}
	module, tag := parts[0], parts[1]
	ns, ok := opencncModel.NamespaceByModule[module]
	return ns, tag, ok
}

func ConvertToXML(data interface{}, buf *bytes.Buffer, indent int) error {
	switch v := data.(type) {
	case map[string]interface{}:
		// Special case for gate-control-entry item
		if _, hasIndex := v["index"]; hasIndex {
			if _, hasOp := v["operation-name"]; hasOp {
				writeIndent(buf, indent)
				buf.WriteString("<gate-control-entry>\n")

				writeElement(buf, "index", v["index"], indent+1)

				if opName, ok := v["operation-name"].(string); ok {
					if parts := strings.SplitN(opName, ":", 2); len(parts) == 2 {
						opName = parts[1]
					}
					writeElement(buf, "operation-name", opName, indent+1)
				}

				writeElement(buf, "time-interval-value", v["time-interval-value"], indent+1)
				writeElement(buf, "gate-states-value", v["gate-states-value"], indent+1)

				writeIndent(buf, indent)
				buf.WriteString("</gate-control-entry>\n")
				return nil
			}
		}

		for k, val := range v {
			ns, tag, found := ResolveNamespace(k)
			open := tag
			if found {
				open = fmt.Sprintf(`%s xmlns="%s"`, tag, ns)
			}

			// Detect list value → repeat tag per array element (avoid extra nesting)
			if arr, ok := val.([]interface{}); ok {
				// Avoid double wrapping for gate-control-entry
				if tag == "gate-control-entry" {
					for _, item := range arr {
						if err := ConvertToXML(item, buf, indent); err != nil {
							return err
						}
					}
				} else {
					for _, item := range arr {
						writeIndent(buf, indent)
						buf.WriteString("<" + open + ">\n")
						if err := ConvertToXML(item, buf, indent+1); err != nil {
							return err
						}
						writeIndent(buf, indent)
						buf.WriteString(fmt.Sprintf("</%s>\n", tag))
					}
				}
				continue
			}

			// For scalar values → single line element
			if isScalar(val) {
				writeElement(buf, tag, val, indent)
				continue
			}

			// For objects → nested block
			writeIndent(buf, indent)
			buf.WriteString("<" + open + ">\n")
			if err := ConvertToXML(val, buf, indent+1); err != nil {
				return err
			}
			writeIndent(buf, indent)
			buf.WriteString(fmt.Sprintf("</%s>\n", tag))
		}

	case []interface{}:
		for _, item := range v {
			if err := ConvertToXML(item, buf, indent); err != nil {
				return err
			}
		}

	// scalar values at top level (rare)
	case string, float64, bool:
		writeElement(buf, "value", v, indent)

	case nil:
		writeIndent(buf, indent)
		buf.WriteString("<nil/>\n")

	default:
		writeIndent(buf, indent)
		buf.WriteString(fmt.Sprintf("<unsupported type=\"%T\"/>\n", v))
	}
	return nil
}

func writeElement(buf *bytes.Buffer, name string, value interface{}, indent int) {
	writeIndent(buf, indent)
	buf.WriteString(fmt.Sprintf("<%s>", name))
	switch v := value.(type) {
	case string:
		buf.WriteString(xmlEscape(v))
	case float64:
		if v == float64(int64(v)) {
			buf.WriteString(strconv.FormatInt(int64(v), 10))
		} else {
			buf.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		}
	case bool:
		buf.WriteString(strconv.FormatBool(v))
	default:
		if v != nil {
			buf.WriteString(fmt.Sprintf("%v", v))
		}
	}
	buf.WriteString(fmt.Sprintf("</%s>\n", name))
}

func writeIndent(buf *bytes.Buffer, n int) {
	buf.WriteString(strings.Repeat("  ", n))
}

func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

func isScalar(v interface{}) bool {
	switch v.(type) {
	case string, float64, bool:
		return true
	default:
		return false
	}
}
