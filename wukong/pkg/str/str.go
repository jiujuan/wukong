package str

import "strings"

type Str string

func S(v string) Str { return Str(v) }

func (s Str) String() string { return string(s) }

func (s Str) Val() string { return string(s) }

func (s Str) Trim() Str { return Str(strings.TrimSpace(string(s))) }

func (s Str) Lower() Str { return Str(strings.ToLower(string(s))) }

func (s Str) Upper() Str { return Str(strings.ToUpper(string(s))) }

func (s Str) Compact() Str { return Str(Compact(string(s))) }

func (s Str) CompressSpace() Str { return Str(CompressSpace(string(s))) }

func (s Str) TrimLower() Str { return Str(TrimLower(string(s))) }

func (s Str) Replace(old, new string) Str {
	return Str(strings.ReplaceAll(string(s), old, new))
}

func (s Str) TrimPrefix(prefix string) Str {
	return Str(strings.TrimPrefix(string(s), prefix))
}

func (s Str) TrimSuffix(suffix string) Str {
	return Str(strings.TrimSuffix(string(s), suffix))
}

func (s Str) Contains(substr string) bool { return strings.Contains(string(s), substr) }

func (s Str) HasPrefix(prefix string) bool { return strings.HasPrefix(string(s), prefix) }

func (s Str) HasSuffix(suffix string) bool { return strings.HasSuffix(string(s), suffix) }

func (s Str) EqFold(other string) bool { return strings.EqualFold(string(s), other) }

func (s Str) Cut(sep string) (string, string, bool) { return strings.Cut(string(s), sep) }

func (s Str) Split(sep string) []string { return Split(string(s), sep) }

func (s Str) Default(fallback string) string {
	if Empty(string(s)) {
		return fallback
	}
	return string(s)
}

func Trim(v string) string { return strings.TrimSpace(v) }

func Lower(v string) string { return strings.ToLower(v) }

func Upper(v string) string { return strings.ToUpper(v) }

func TrimLower(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

func Compact(v string) string { return strings.Join(strings.Fields(v), " ") }

func CompressSpace(v string) string { return Compact(strings.TrimSpace(v)) }

func Norm(v string) string { return TrimLower(v) }

func Clean(v string) string { return CompressSpace(v) }

func Empty(v string) bool { return strings.TrimSpace(v) == "" }

func NotEmpty(v string) bool { return !Empty(v) }

func Eq(a, b string) bool { return a == b }

func EqFold(a, b string) bool { return strings.EqualFold(a, b) }

func Contains(v, substr string) bool { return strings.Contains(v, substr) }

func HasPrefix(v, prefix string) bool { return strings.HasPrefix(v, prefix) }

func HasSuffix(v, suffix string) bool { return strings.HasSuffix(v, suffix) }

func ReplaceAll(v, old, new string) string { return strings.ReplaceAll(v, old, new) }

func Cut(v, sep string) (string, string, bool) { return strings.Cut(v, sep) }

func Split(v, sep string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, sep)
}

func Join(sep string, parts ...string) string { return strings.Join(parts, sep) }
