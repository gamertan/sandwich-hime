// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import "testing"

func FuzzCompileNeverPanics(f *testing.F) {
	for _, seed := range []string{
		profileSource,
		"",
		"<?sando go\npackage p\nfunc F()\n?>",
		"<?sando go\npackage p\nfunc F(v string)\n?>\n<a href=\"<?= v ?>\">x</a>",
		"<?sando go\npackage p\nfunc F()\n?>\n<script><?= `?>` ?></script>",
		"<?sando go\npackage p\nfunc F(v string)\n?>\n<script><!--<script></script>\n<?= v ?>\n<!--\n</script>\n-->",
		"\xef\xbb\xbf\r\n<?sando go\r\npackage p\r\nfunc F()\r\n?>\r\n<p>x</p>",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = Compile("fuzz.sando", []byte(source))
	})
}

func FuzzGoDelimiterNeverPanics(f *testing.F) {
	for _, seed := range []string{`?>`, `"?>" ?>`, "`?>` ?>", `/* ?> */ ?>`, "// ?>\n?>", `'?' ?>`} {
		f.Add(seed, uint8(0))
	}
	f.Fuzz(func(t *testing.T, source string, start uint8) {
		offset := int(start)
		if offset > len(source) {
			offset = len(source)
		}
		result := findGoDelimiter([]byte(source), offset)
		if result < -1 || result > len(source) {
			t.Fatalf("invalid delimiter offset %d for %d bytes", result, len(source))
		}
	})
}
