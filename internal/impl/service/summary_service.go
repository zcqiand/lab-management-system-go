package service

// summary_service.go — M05 报告汇总 + 仪表盘统计（REQ-2026-004 Task 11；
// rails Summary::SummaryService + MaterialRates 镜像）。按 tenant +
// categoryCode（'ALL' = 不过滤）过滤 sample_receipts，commissionDate 字典序当
// 日期序；stats = 3 总数 + 3 桶状态 + 任务计数 + 4 段扩展（M05.F01.I03/I04）。

import (
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SummaryService 汇总面（tenant 由方法参注入）。
type SummaryService struct {
	DB *gorm.DB
}

// NewSummaryService 构造。
func NewSummaryService(db *gorm.DB) *SummaryService {
	return &SummaryService{DB: db}
}

// summaryScope 汇总查询：tenant + 可选 categoryCode（ALL/空 = 不过滤）+ 可选
// commissionDate 前后界（YYYY-MM-DD 字典序 = 日期序）；排序 commissionDate
// DESC, commissionCode。
func (s *SummaryService) summaryScope(tenantID, categoryCode, dateFrom, dateTo string) ([]map[string]any, error) {
	q := s.DB.Table("sample_receipts").Where("tenant_id = ?", tenantID)
	if categoryCode != "" && categoryCode != "ALL" {
		q = q.Where("category_code = ?", categoryCode)
	}
	if dateFrom != "" {
		q = q.Where("commission_date >= ?", dateFrom)
	}
	if dateTo != "" {
		q = q.Where("commission_date <= ?", dateTo)
	}
	var rows []map[string]any
	if err := q.Order("commission_date desc, commission_code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ReportSummary 报告汇总表。
func (s *SummaryService) ReportSummary(tenantID, categoryCode, dateFrom, dateTo string) (map[string]any, error) {
	rows, err := s.summaryScope(tenantID, categoryCode, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	name := "ALL"
	if categoryCode != "" {
		name = categoryCode
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"commissionCode": strVal(r["commission_code"]),
			"categoryCode":   strVal(r["category_code"]),
			"projectName":    strVal(r["project_name"]),
			"flowStatus":     strVal(r["flow_status"]),
			"result":         strVal(r["result"]),
			"reportCode":     strVal(r["report_code"]),
		})
	}
	return map[string]any{
		"summaryName": "报告汇总（" + name + "）",
		"columns":     SummaryColumns(),
		"rows":        out,
	}, nil
}

// SummaryColumns 报告汇总 6 列（camel 键直出）。
func SummaryColumns() []map[string]any {
	return []map[string]any{
		{"key": "commissionCode", "label": "委托编号"},
		{"key": "categoryCode", "label": "报告类别"},
		{"key": "projectName", "label": "工程名称"},
		{"key": "flowStatus", "label": "流程状态"},
		{"key": "result", "label": "结论"},
		{"key": "reportCode", "label": "报告编号"},
	}
}

// DashboardStats 仪表盘（3 总数 + 3 桶 + pending/today + 合格率/产出/漏斗）。
func (s *SummaryService) DashboardStats(tenantID string) (map[string]any, error) {
	rows, err := s.summaryScope(tenantID, "ALL", "", "")
	if err != nil {
		return nil, err
	}
	var contractCount, sampleCount int64
	if err := s.DB.Table("contracts").Where("tenant_id = ?", tenantID).
		Count(&contractCount).Error; err != nil {
		return nil, err
	}
	if err := s.DB.Table("samples").Where("tenant_id = ?", tenantID).
		Count(&sampleCount).Error; err != nil {
		return nil, err
	}
	bucket := func(stages ...string) int {
		n := 0
		for _, r := range rows {
			fs := strVal(r["flow_status"])
			for _, st := range stages {
				if fs == st {
					n++
					break
				}
			}
		}
		return n
	}
	countIf := func(pred func(map[string]any) bool) int {
		n := 0
		for _, r := range rows {
			if pred(r) {
				n++
			}
		}
		return n
	}
	hasReportCode := func(r map[string]any) bool { return r["report_code"] != nil }
	today := time.Now().UTC().Format("2006-01-02")
	todayTest := countIf(func(r map[string]any) bool {
		return strings.HasPrefix(strVal(r["created_at"]), today) ||
			strings.HasPrefix(strVal(r["test_start_date"]), today)
	})
	rates, err := s.qualifiedRateByMaterial(rows)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"contractCount": contractCount, "receiptCount": len(rows),
		"sampleCount": sampleCount,
		"reportCountByStatus": map[string]any{
			"draft":     bucket("receiving", "task_assignment", "data_entry"),
			"reviewing": bucket("review", "approval"),
			"issued":    bucket("issuance", "archived"),
		},
		"pendingTaskCount":        bucket("task_assignment", "data_entry", "review"),
		"todayTestCount":          todayTest,
		"qualifiedRateByMaterial": rates,
		"reportOutputByStatus": map[string]any{
			"generated": countIf(hasReportCode),
			"pending":   bucket("review", "approval"),
			"issued":    bucket("issuance", "archived"),
		},
		"funnelByStage": map[string]any{
			"pending_collect": bucket("receiving"),
			"received":        bucket("task_assignment"),
			// testing/reporting 按 data_entry 是否已有 report_code 拆分（6 段漏斗）
			"testing":   countIf(func(r map[string]any) bool { return strVal(r["flow_status"]) == "data_entry" && !hasReportCode(r) }),
			"reporting": countIf(func(r map[string]any) bool { return strVal(r["flow_status"]) == "data_entry" && hasReportCode(r) }),
			"reviewing": bucket("review", "approval"),
			"issued":    bucket("issuance", "archived"),
		},
	}, nil
}

// materialKeywords summaryName → 材料类型归类（与 msw handlers-extra 同款语义）。
var materialKeywords = map[string][]string{
	"concrete": {"混凝土", "水泥"},
	"rebar":    {"钢筋", "钢材", "焊接", "机械连接", "连接"},
	"sand":     {"砂", "碎（卵）石", "轻集料", "颗粒级配"},
}

// qualifiedRateByMaterial 码表全量预载按 summaryName 关键词归类，rate 三位小数。
func (s *SummaryService) qualifiedRateByMaterial(rows []map[string]any) (map[string]any, error) {
	var rnRows []map[string]any
	if err := s.DB.Table("inspection_report_names").
		Select("code, COALESCE(summary_name, '') AS summary_name").
		Find(&rnRows).Error; err != nil {
		return nil, err
	}
	summaryNameByCode := map[string]string{}
	for _, r := range rnRows {
		summaryNameByCode[strVal(r["code"])] = strVal(r["summary_name"])
	}
	total := map[string]int{"concrete": 0, "rebar": 0, "sand": 0}
	pass := map[string]int{"concrete": 0, "rebar": 0, "sand": 0}
	for _, r := range rows {
		mat := materialOf(strVal(r["category_code"]), summaryNameByCode)
		if mat == "" {
			continue
		}
		total[mat]++
		if strVal(r["result"]) == "pass" {
			pass[mat]++
		}
	}
	out := map[string]any{}
	for _, mat := range []string{"concrete", "rebar", "sand"} {
		rate := 0.0
		if total[mat] > 0 {
			rate = math.Round(float64(pass[mat])*1000.0/float64(total[mat])) / 1000.0
		}
		out[mat] = map[string]any{"total": total[mat], "pass": pass[mat], "rate": rate}
	}
	return out, nil
}

// materialOf 关键词归类（未命中 → 空串）。
func materialOf(categoryCode string, summaryNameByCode map[string]string) string {
	summaryName := summaryNameByCode[categoryCode]
	for _, mat := range []string{"concrete", "rebar", "sand"} {
		for _, kw := range materialKeywords[mat] {
			if strings.Contains(summaryName, kw) {
				return mat
			}
		}
	}
	return ""
}
