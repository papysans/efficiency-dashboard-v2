package governance

import "testing"

func TestNormalizeDocExtensions(t *testing.T) {
	got := normalizeDocExtensions([]string{".MD", "mdx", "  .Markdown ", "", "  "})
	want := []string{".md", ".mdx", ".markdown"}
	if len(got) != len(want) {
		t.Fatalf("normalize len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalize[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIsDocOnlyCommit(t *testing.T) {
	exts := normalizeDocExtensions([]string{".md", ".mdx", ".markdown"})

	cases := []struct {
		name    string
		touched string
		want    bool
	}{
		{"single md", `["README.md"]`, true},
		{"multi doc mixed exts", `["docs/a.md", "guide.mdx", "CHANGELOG.markdown"]`, true},
		{"uppercase ext", `["README.MD", "Notes.Md"]`, true},
		{"nested path md", `["a/b/c/design.md"]`, true},

		// mixed：含代码文件 → 不排除
		{"mixed py + md", `["main.py", "README.md"]`, false},
		{"mixed go + md", `["service.go", "docs/x.md"]`, false},

		// .txt 守卫：真实数据里的 requirements/CMakeLists/构建输出，绝不误排
		{"requirements.txt", `["backend/requirements.txt"]`, false},
		{"CMakeLists.txt", `["src/model/CMakeLists.txt"]`, false},
		{"build outputs", `["build_output.txt", "build_output2.txt"]`, false},
		{"txt with md mixed", `["notes.txt", "README.md"]`, false},

		// 边界：空列表 / 无 touched_files / 非数组 / 无后缀文件
		{"empty array", `[]`, false},
		{"empty string", ``, false},
		{"null", `null`, false},
		{"malformed json", `not-json`, false},
		{"extensionless file", `["Makefile"]`, false},
		{"empty element only", `["", "  "]`, false},
	}
	for _, tc := range cases {
		if got := IsDocOnlyCommit(tc.touched, exts); got != tc.want {
			t.Errorf("%s: IsDocOnlyCommit(%q) = %v, want %v", tc.name, tc.touched, got, tc.want)
		}
	}
}

func TestIsDocOnlyCommit_DisabledWhenNoExts(t *testing.T) {
	// 空后缀列表 = 规则禁用，任何 commit 都不命中
	if IsDocOnlyCommit(`["README.md"]`, nil) {
		t.Fatal("empty exts should disable the rule, but README.md matched")
	}
}
