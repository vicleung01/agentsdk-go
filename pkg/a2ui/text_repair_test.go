package a2ui

import (
	"strings"
	"testing"
)

func TestRepairTextPresentationCollapsedLsFence(t *testing.T) {
	input := "`ls` 命令执行完成\n```text attachments bing.html bing2.html inspection-report prompt ```"
	got := repairTextPresentation(input)
	if !strings.Contains(got, "attachments\nbing.html") {
		t.Fatalf("expected newline-separated listing, got %q", got)
	}
}

func TestRepairTextPresentationRealLsOutput(t *testing.T) {
	body := "attachments bing.html bing2.html inspection-report m_boss.html multi_search.py parse_bing.py prompt redis-cache search_boss.py server_inspection_report.html 产品规划文档 前端开发文档 架构设计文档"
	input := "`ls` 命令执行完成\n```text " + body + " ```"
	got := repairTextPresentation(input)
	if !strings.Contains(got, "bing.html\nbing2.html") {
		t.Fatalf("expected multiline ls output, got %q", got)
	}
	if strings.Contains(got, "bing.html bing2.html") {
		t.Fatalf("expected spaces between files removed, got %q", got)
	}
}

func TestRepairTextPresentationLsAlhCollapsedOnOneLine(t *testing.T) {
	body := "total 504 drwxr-x---@ 18 zhanbei staff 576B Jun 12 11:30 . drwx------@ 14 zhanbei staff 448B Jun 11 21:24 .."
	input := "命令已执行：`ls -alh` 输出如下：\n```text " + body + " ```"
	got := repairTextPresentation(input)
	if !strings.Contains(got, "```text\ntotal 504\n") {
		t.Fatalf("expected normalized fence with total line, got %q", got)
	}
	if !strings.Contains(got, "drwxr-x---@ 18 zhanbei staff 576B Jun 12 11:30 .") {
		t.Fatalf("expected preserved ls entry, got %q", got)
	}
	if strings.Contains(got, "total 504 drwxr-x---@") {
		t.Fatalf("expected split between total and entries, got %q", got)
	}
}

func TestRepairTextPresentationPreservesExistingNewlines(t *testing.T) {
	input := "```text\nline1\nline2\n```"
	got := repairTextPresentation(input)
	if got != input {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestCollapseSpacesToLinesSkipsProse(t *testing.T) {
	got := collapseSpacesToLines("this is a normal sentence with many words but not a listing")
	if got != "this is a normal sentence with many words but not a listing" {
		t.Fatalf("unexpected change: %q", got)
	}
}
