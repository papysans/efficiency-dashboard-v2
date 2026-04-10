package macro

// GDPData GDP数据
type GDPData struct {
	ReportDate string
	TotalGDP   float64 // 总量亿元
	GDPYoY     float64 // GDP同比%
	SecondYoY  float64 // 第二产业同比%
	ThirdYoY   float64 // 第三产业同比%
}

// CPIData CPI数据
type CPIData struct {
	ReportDate  string
	Time        string
	NationalYoY float64 // 同比%
	NationalSeq float64 // 环比%
}

// PMIData PMI数据
type PMIData struct {
	ReportDate          string
	Time                string
	ManufacturingPMI    float64 // 制造业PMI
	NonManufacturingPMI float64 // 非制造业PMI
}

// PPIData PPI数据
type PPIData struct {
	ReportDate string
	Time       string
	PPIYoY     float64 // 同比%
}

// M2Data M2数据
type M2Data struct {
	ReportDate string
	Time       string
	M2Yoy      float64 // M2同比%
	M1Yoy      float64 // M1同比%
}

// RMBLoanData 人民币贷款数据
type RMBLoanData struct {
	ReportDate string
	Time       string
	NewLoan    float64 // 当月新增亿元
	LoanYoY    float64 // 同比%
	LoanSeq    float64 // 环比%
	Accumulate float64 // 年累计亿元
	AccYoY     float64 // 累计同比%
}

// MacroSummary 聚合所有宏观数据
type MacroSummary struct {
	GDPs  []GDPData
	CPIs  []CPIData
	PMIs  []PMIData
	PPIs  []PPIData
	M2s   []M2Data
	Loans []RMBLoanData
}
