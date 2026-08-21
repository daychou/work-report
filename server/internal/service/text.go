package service

import (
	"regexp"
	"strings"
)

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

var htmlEntities = strings.NewReplacer(
	"&nbsp;", " ",
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", `"`,
	"&#39;", "'",
)

// StripHTML 去掉富文本 HTML 标签并压缩空白，提取纯文本
// （任务详细内容改为富文本后，AI 提示词与 Markdown 报表导出用）
func StripHTML(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = htmlEntities.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}
