package index

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	rePackage   = regexp.MustCompile(`^\s*package\s+([\w.]+)\s*;`)
	reGoPackage = regexp.MustCompile(`^\s*option\s+go_package\s*=\s*"([^";]+)(?:;(\w+))?"\s*;`)
	reService   = regexp.MustCompile(`^\s*service\s+(\w+)\s*\{`)
	reRPC       = regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*(?:stream\s+)?([\w.]+)\s*\)\s*returns\s*\(\s*(?:stream\s+)?([\w.]+)\s*\)`)
	reMessage   = regexp.MustCompile(`^\s*message\s+(\w+)\s*\{`)
	reEnum      = regexp.MustCompile(`^\s*enum\s+(\w+)\s*\{`)
	reField     = regexp.MustCompile(`(repeated\s+)?([\w.]+)\s+(\w+)\s*=\s*(\d+)\s*;`)
	reEnumValue = regexp.MustCompile(`(\w+)\s*=\s*\d+\s*;`)
)

// ParseProto reads the declarations of a .proto file. It understands the subset of proto3 that service
// contracts use: package, go_package, services with unary or streaming RPCs, messages (also on one line),
// enums and the comments directly above each.
func ParseProto(src string) *ProtoFile {
	pf := &ProtoFile{}
	lines := strings.Split(src, "\n")
	var doc []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		n := i + 1
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "//") {
			doc = append(doc, strings.TrimSpace(strings.TrimPrefix(t, "//")))
			continue
		}
		if m := rePackage.FindStringSubmatch(line); m != nil {
			pf.Package = m[1]
		} else if m := reGoPackage.FindStringSubmatch(line); m != nil {
			pf.GoPackage = m[1]
			pf.GoName = m[2]
			if pf.GoName == "" {
				pf.GoName = m[1][strings.LastIndex(m[1], "/")+1:]
			}
		} else if m := reService.FindStringSubmatch(line); m != nil {
			end := blockEnd(lines, i)
			svc := ProtoService{Name: m[1], Doc: strings.Join(doc, " "), Span: Span{n, end + 1}}
			for j := i; j <= end; j++ {
				if r := reRPC.FindStringSubmatch(lines[j]); r != nil {
					svc.RPCs = append(svc.RPCs, ProtoRPC{Name: r[1], Request: last(r[2]), Response: last(r[3]), Line: j + 1})
				}
			}
			pf.Services = append(pf.Services, svc)
			i = end
		} else if m := reMessage.FindStringSubmatch(line); m != nil {
			end := blockEnd(lines, i)
			msg := ProtoMessage{Name: m[1], Doc: strings.Join(doc, " "), Span: Span{n, end + 1}}
			for j := i; j <= end; j++ {
				body := lines[j]
				if j == i {
					body = body[strings.Index(body, "{")+1:]
				}
				for _, f := range reField.FindAllStringSubmatch(body, -1) {
					num, _ := strconv.Atoi(f[4])
					msg.Fields = append(msg.Fields, ProtoField{Name: f[3], Type: last(f[2]), Number: num, Repeated: f[1] != "", Line: j + 1})
				}
			}
			pf.Messages = append(pf.Messages, msg)
			i = end
		} else if m := reEnum.FindStringSubmatch(line); m != nil {
			end := blockEnd(lines, i)
			en := ProtoEnum{Name: m[1], Span: Span{n, end + 1}}
			for j := i; j <= end; j++ {
				body := lines[j]
				if j == i {
					body = body[strings.Index(body, "{")+1:]
				}
				for _, v := range reEnumValue.FindAllStringSubmatch(body, -1) {
					en.Values = append(en.Values, v[1])
				}
			}
			pf.Enums = append(pf.Enums, en)
			i = end
		}
		if t != "" {
			doc = nil
		}
	}
	return pf
}

// blockEnd returns the index of the line that closes the block opened on line i.
func blockEnd(lines []string, i int) int {
	depth := 0
	for j := i; j < len(lines); j++ {
		depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
		if depth <= 0 {
			return j
		}
	}
	return len(lines) - 1
}

func last(qualified string) string { return qualified[strings.LastIndex(qualified, ".")+1:] }

// GoFieldName is the Go name protoc-gen-go gives a field: eta_seconds -> EtaSeconds.
func GoFieldName(name string) string {
	var b strings.Builder
	up := true
	for _, r := range name {
		if r == '_' {
			up = true
			continue
		}
		if up {
			b.WriteString(strings.ToUpper(string(r)))
			up = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
