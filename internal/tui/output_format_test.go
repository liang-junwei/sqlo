package tui

import "testing"

// TestMetaOutputFormat 验证新增的 \o 元命令解析
func TestMetaOutputFormat(t *testing.T) {
	cases := []struct {
		line     string
		wantKind metaKind
		wantText string
		wantErr  bool
	}{
		{`\o`, metaFormat, "table", false},
		{`\o json`, metaFormat, "json", false},
		{`\o csv`, metaFormat, "csv", false},
		{`\o yaml`, metaFormat, "yaml", false},
		{`\o bogus`, metaNone, "", true},
	}
	for _, c := range cases {
		act, err := parseMeta(c.line, "mysql", "test")
		if c.wantErr {
			if err == nil {
				t.Errorf("%q: 期望解析错误，实际无错误", c.line)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: 意外错误 %v", c.line, err)
		}
		if act.kind != c.wantKind {
			t.Errorf("%q: kind=%v 期望 %v", c.line, act.kind, c.wantKind)
		}
		if act.text != c.wantText {
			t.Errorf("%q: text=%q 期望 %q", c.line, act.text, c.wantText)
		}
	}
}
