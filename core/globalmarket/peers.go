package globalmarket

var peerMap = map[string]PeerInfo{
	"sz300454": {
		Peers:         []string{"crwd.us", "panw.us", "ftnt.us"},
		PeerNames:     []string{"CrowdStrike", "Palo Alto", "Fortinet"},
		SectorETF:     "cibr.us",
		SectorETFName: "CIBR",
		Category:      "网络安全",
	},
	"sz002594": {
		Peers:         []string{"tsla.us", "nio.us", "li.us"},
		PeerNames:     []string{"Tesla", "NIO", "Li Auto"},
		SectorETF:     "driv.us",
		SectorETFName: "DRIV",
		Category:      "新能源汽车",
	},
	"sz300750": {
		Peers:         []string{"tsla.us", "alb.us", "lthm.us"},
		PeerNames:     []string{"Tesla", "Albemarle", "Livent"},
		SectorETF:     "lit.us",
		SectorETFName: "LIT",
		Category:      "动力电池",
	},
	"sh600036": {
		Peers:         []string{"jpm.us", "bac.us", "c.us"},
		PeerNames:     []string{"JPMorgan", "BankOfAmerica", "Citigroup"},
		SectorETF:     "kbe.us",
		SectorETFName: "KBE",
		Category:      "银行",
	},
	"sh601398": {
		Peers:         []string{"jpm.us", "bac.us"},
		PeerNames:     []string{"JPMorgan", "BankOfAmerica"},
		SectorETF:     "kbe.us",
		SectorETFName: "KBE",
		Category:      "银行",
	},
	"sh601318": {
		Peers:         []string{"met.us", "pru.us"},
		PeerNames:     []string{"MetLife", "Prudential"},
		SectorETF:     "kie.us",
		SectorETFName: "KIE",
		Category:      "保险",
	},
	"sh600519": {
		Peers:         []string{"bf-b.us", "deo.us"},
		PeerNames:     []string{"BrownForman", "Diageo"},
		SectorETF:     "",
		SectorETFName: "",
		Category:      "白酒",
	},
	"sh603259": {
		Peers:         []string{"iqvia.us", "crl.us"},
		PeerNames:     []string{"IQVIA", "CharlesRiverLab"},
		SectorETF:     "xbi.us",
		SectorETFName: "XBI",
		Category:      "医药CRO",
	},
	"sz000063": {
		Peers:         []string{"csco.us", "eric.us"},
		PeerNames:     []string{"Cisco", "Ericsson"},
		SectorETF:     "ign.us",
		SectorETFName: "IGN",
		Category:      "通信设备",
	},
	"sh600900": {
		Peers:         []string{"nee.us", "d.us"},
		PeerNames:     []string{"NextEraEnergy", "Dominion"},
		SectorETF:     "tan.us",
		SectorETFName: "TAN",
		Category:      "电力",
	},
	"sh601012": {
		Peers:         []string{"enph.us", "sedg.us"},
		PeerNames:     []string{"Enphase", "SolarEdge"},
		SectorETF:     "tan.us",
		SectorETFName: "TAN",
		Category:      "光伏",
	},
	"sh688981": {
		Peers:         []string{"intc.us", "nvda.us", "tsm.us"},
		PeerNames:     []string{"Intel", "NVIDIA", "TSMC"},
		SectorETF:     "soxx.us",
		SectorETFName: "SOXX",
		Category:      "半导体",
	},
	"sz002415": {
		Peers:         []string{"axon.us", "msi.us"},
		PeerNames:     []string{"Axon", "Motorola"},
		SectorETF:     "",
		SectorETFName: "",
		Category:      "安防",
	},
	"sh600276": {
		Peers:         []string{"mrk.us", "pfe.us"},
		PeerNames:     []string{"Merck", "Pfizer"},
		SectorETF:     "xbi.us",
		SectorETFName: "XBI",
		Category:      "医药",
	},
	"sh688599": {
		Peers:         []string{"enph.us", "sedg.us"},
		PeerNames:     []string{"Enphase", "SolarEdge"},
		SectorETF:     "tan.us",
		SectorETFName: "TAN",
		Category:      "光伏",
	},
	"sz002230": {
		Peers:         []string{"amzn.us", "googl.us"},
		PeerNames:     []string{"Amazon", "Alphabet"},
		SectorETF:     "socl.us",
		SectorETFName: "SOCL",
		Category:      "互联网",
	},
	"hk00700": {
		Peers:         []string{"meta.us", "googl.us"},
		PeerNames:     []string{"Meta", "Alphabet"},
		SectorETF:     "socl.us",
		SectorETFName: "SOCL",
		Category:      "互联网",
	},
}

var dualListedMap = map[string]string{
	"sh688688": "baba.us",
	"sh601857": "e.us",
}

// GlobalIndices 全球6大指数
var GlobalIndices = []IndexDef{
	{Symbol: "%5Espx", Name: "标普500"},
	{Symbol: "%5Endq", Name: "纳斯达克"},
	{Symbol: "%5Edji", Name: "道琼斯"},
	{Symbol: "%5Ehsi", Name: "恒生指数"},
	{Symbol: "%5Edax", Name: "DAX德国"},
	{Symbol: "%5Eftse", Name: "FTSE英国"},
}

// GetPeerInfo 获取公司的对标信息
func GetPeerInfo(companyID string) (PeerInfo, bool) {
	info, ok := peerMap[companyID]
	if ok {
		if usCode, found := dualListedMap[companyID]; found {
			info.DualListedUS = usCode
		}
		return info, true
	}

	if usCode, found := dualListedMap[companyID]; found {
		return PeerInfo{
			Peers:        []string{usCode},
			PeerNames:    []string{},
			DualListedUS: usCode,
		}, true
	}

	return PeerInfo{}, false
}
