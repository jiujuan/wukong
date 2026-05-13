package str

import "testing"

func TestStrChainAndHelpers(t *testing.T) {
	if got := S("  Hello World  ").Trim().Lower().Val(); got != "hello world" {
		t.Fatalf("chain result = %q", got)
	}
	if got := S("  a   b   c  ").CompressSpace().Val(); got != "a b c" {
		t.Fatalf("clean result = %q", got)
	}
	if got := S("foo-bar").Replace("-", "_").Val(); got != "foo_bar" {
		t.Fatalf("replace result = %q", got)
	}
	if got := S("abc").Default("x"); got != "abc" {
		t.Fatalf("default result = %q", got)
	}
	if got := S("   ").Default("x"); got != "x" {
		t.Fatalf("default fallback = %q", got)
	}
	if before, after, ok := S("a=b").Cut("="); !ok || before != "a" || after != "b" {
		t.Fatalf("cut result = %q %q %v", before, after, ok)
	}
	if got := Join("/", "a", "b", "c"); got != "a/b/c" {
		t.Fatalf("join result = %q", got)
	}
}

func TestTopLevelStringHelpers(t *testing.T) {
	if Trim("  x  ") != "x" || Lower("ABC") != "abc" || Upper("abc") != "ABC" {
		t.Fatal("basic transforms failed")
	}
	if TrimLower("  AbC  ") != "abc" {
		t.Fatalf("trim lower result = %q", TrimLower("  AbC  "))
	}
	if !Empty("   ") || NotEmpty("x") == false {
		t.Fatal("empty helpers failed")
	}
	if !EqFold("Hello", "hello") || Eq("a", "b") {
		t.Fatal("comparison helpers failed")
	}
	if !Contains("hello world", "world") || !HasPrefix("hello", "he") || !HasSuffix("hello", "lo") {
		t.Fatal("match helpers failed")
	}
	if got := Compact("a   b\tc"); got != "a b c" {
		t.Fatalf("compact result = %q", got)
	}
	if got := CompressSpace(" a   b "); got != "a b" {
		t.Fatalf("compress space result = %q", got)
	}
	if got := S("  AbC  ").TrimLower().Val(); got != "abc" {
		t.Fatalf("trim lower chain result = %q", got)
	}
	if parts := Split("a,b,c", ","); len(parts) != 3 || parts[1] != "b" {
		t.Fatalf("split result = %#v", parts)
	}
}
