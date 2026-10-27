package saas

// menu_mapper_test.go — saas EffectiveMenuNode → lab 契约 MenuNode 递归映射
// （镜像 rails SaasMenuMapper：label=title||code、icon 按 type 兜底、sortOrder 升序）。

import (
	"encoding/json"
	"testing"
)

func mustJSON(t *testing.T, s string) []any {
	t.Helper()
	var v []any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("test json: %v", err)
	}
	return v
}

func TestMenuMapperMapsTitleIconChildren(t *testing.T) {
	roots := mustJSON(t, `[
		{"id":"m2","title":"报表","code":"report","type":"group","sortOrder":2,
		 "children":[{"id":"m2-1","title":"月报","code":"monthly","type":"page","sortOrder":1,"path":"/reports/monthly"}]},
		{"id":"m1","code":"home","type":"page","sortOrder":1,"path":"/"}
	]`)
	out := NewMenuMapper().Map(roots)
	if len(out) != 2 {
		t.Fatalf("应 2 根: %v", out)
	}
	first := out[0]
	if first["id"] != "m1" {
		t.Fatalf("sortOrder 升序应 m1 先: %v", first)
	}
	// 无 title → label 回退 code；page 无 icon → file
	if first["label"] != "home" || first["icon"] != "file" {
		t.Fatalf("label/icon 回退不符: %v", first)
	}
	if first["path"] != "/" {
		t.Fatalf("path 应透传: %v", first)
	}
	second := out[1]
	if second["label"] != "报表" || second["icon"] != "resource" {
		t.Fatalf("group icon 兜底不符: %v", second)
	}
	children := second["children"].([]map[string]any)
	if len(children) != 1 || children[0]["label"] != "月报" || children[0]["path"] != "/reports/monthly" {
		t.Fatalf("children 递归不符: %v", children)
	}
}

func TestMenuMapperEmptyRoots(t *testing.T) {
	if out := NewMenuMapper().Map(nil); len(out) != 0 {
		t.Fatalf("空入参应空切片: %v", out)
	}
	if out := NewMenuMapper().Map([]any{}); len(out) != 0 {
		t.Fatalf("空入参应空切片: %v", out)
	}
}
