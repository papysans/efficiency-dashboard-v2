package htmlreport

const reportTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}} - 回测报告</title>
<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
<style>
  body { font-family: 'Microsoft YaHei', Arial, sans-serif; background: #1a1a2e; color: #e0e0e0; margin: 0; padding: 20px; }
  h1 { color: #00d4ff; text-align: center; margin-bottom: 30px; }
  .chart-container { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-bottom: 20px; }
  .chart-box { background: #16213e; border-radius: 8px; padding: 15px; }
  .chart-box h3 { color: #00d4ff; margin: 0 0 10px 0; font-size: 14px; }
  .chart { width: 100%; height: 350px; }
  .full-width { grid-column: 1 / -1; }
</style>
</head>
<body>
<h1>{{.Title}} 回测分析报告</h1>
<div class="chart-container">
  <div class="chart-box full-width">
    <h3>权益曲线 &amp; 回撤</h3>
    <div id="equityChart" class="chart"></div>
  </div>
  <div class="chart-box">
    <h3>月度收益热力图</h3>
    <div id="monthlyChart" class="chart"></div>
  </div>
  <div class="chart-box">
    <h3>交易盈亏散点图（持仓天数 vs 盈亏%）</h3>
    <div id="tradesChart" class="chart"></div>
  </div>
  <div class="chart-box full-width">
    <h3>各策略总收益率对比</h3>
    <div id="strategyChart" class="chart" style="height:250px;"></div>
  </div>
</div>
<div class="chart-container">
  <div class="chart-box">
    <h3>滚动 Sharpe 曲线（60日窗口）</h3>
    <div id="sharpeChart" class="chart"></div>
  </div>
  <div class="chart-box">
    <h3>策略多维气泡图（X=最大回撤, Y=总收益, 气泡=Sharpe）</h3>
    <div id="bubbleChart" class="chart"></div>
  </div>
  <div class="chart-box">
    <h3>盈亏分布直方图</h3>
    <div id="pnlHistChart" class="chart"></div>
  </div>
  <div class="chart-box">
    <h3>持仓天数分布直方图</h3>
    <div id="holdDaysChart" class="chart"></div>
  </div>
</div>
<script>
var equityData = {{.EquityCurveData}};
var monthlyData = {{.MonthlyReturns}};
var tradesData = {{.TradesData}};
var strategyData = {{.StrategyCompare}};

// 权益曲线图
var equityChart = echarts.init(document.getElementById('equityChart'));
var dates = equityData.map(function(d){ return d.date; });
var values = equityData.map(function(d){ return d.value; });
var drawdowns = equityData.map(function(d){ return d.drawdown; });
var benchmarkValues = equityData.map(function(d){ return d.benchmarkValue; });
equityChart.setOption({
  backgroundColor: 'transparent',
  tooltip: { trigger: 'axis', axisPointer: { type: 'cross' } },
  legend: { data: ['组合价值', '买入持有', '回撤%'], textStyle: { color: '#e0e0e0' } },
  grid: [{ left: '5%', right: '5%', top: '10%', height: '55%' }, { left: '5%', right: '5%', top: '70%', height: '20%' }],
  xAxis: [{ type: 'category', data: dates, gridIndex: 0, axisLabel: { color: '#e0e0e0' } }, { type: 'category', data: dates, gridIndex: 1, axisLabel: { color: '#e0e0e0' } }],
  yAxis: [{ type: 'value', name: '资金(元)', gridIndex: 0, axisLabel: { color: '#e0e0e0' } }, { type: 'value', name: '回撤%', gridIndex: 1, axisLabel: { color: '#e0e0e0' }, inverse: true }],
  series: [
    { name: '组合价值', type: 'line', data: values, smooth: true, lineStyle: { color: '#00d4ff', width: 2 }, areaStyle: { color: 'rgba(0,212,255,0.1)' }, xAxisIndex: 0, yAxisIndex: 0 },
    { name: '买入持有', type: 'line', data: benchmarkValues, smooth: true, lineStyle: { color: '#909399', width: 1.5, type: 'dashed' }, itemStyle: { color: '#909399' }, xAxisIndex: 0, yAxisIndex: 0 },
    { name: '回撤%', type: 'bar', data: drawdowns, itemStyle: { color: 'rgba(255,100,100,0.6)' }, xAxisIndex: 1, yAxisIndex: 1 }
  ]
});

// 月度收益热力图
var monthlyChart = echarts.init(document.getElementById('monthlyChart'));
var months = ['1月','2月','3月','4月','5月','6月','7月','8月','9月','10月','11月','12月'];
var years = [];
var heatData = [];
monthlyData.forEach(function(d){
  if(years.indexOf(d.year) === -1) years.push(d.year);
  heatData.push([months.indexOf(d.month+'月'), years.indexOf(d.year), parseFloat(d.return.toFixed(2))]);
});
monthlyChart.setOption({
  backgroundColor: 'transparent',
  tooltip: { position: 'top', formatter: function(p){ return p.data[2]+'%'; } },
  grid: { left: '10%', right: '5%', top: '10%', bottom: '10%' },
  xAxis: { type: 'category', data: months, axisLabel: { color: '#e0e0e0' } },
  yAxis: { type: 'category', data: years.map(String), axisLabel: { color: '#e0e0e0' } },
  visualMap: { min: -10, max: 10, calculable: true, orient: 'horizontal', left: 'center', bottom: '0%', inRange: { color: ['#d73027','#ffffbf','#1a9850'] }, textStyle: { color: '#e0e0e0' } },
  series: [{ type: 'heatmap', data: heatData, label: { show: true, fontSize: 10, formatter: function(p){ return p.data[2]; } } }]
});

// 交易散点图
var tradesChart = echarts.init(document.getElementById('tradesChart'));
var scatterData = tradesData.map(function(d){ return [d.holdDays, parseFloat(d.pnlPct.toFixed(2)), d.strategyName]; });
tradesChart.setOption({
  backgroundColor: 'transparent',
  tooltip: { formatter: function(p){ return '策略:'+p.data[2]+'<br>持仓:'+p.data[0]+'天<br>盈亏:'+p.data[1]+'%'; } },
  xAxis: { name: '持仓天数', axisLabel: { color: '#e0e0e0' } },
  yAxis: { name: '盈亏%', axisLabel: { color: '#e0e0e0' } },
  series: [{ type: 'scatter', data: scatterData, symbolSize: 8, itemStyle: { color: function(p){ return p.data[1] >= 0 ? '#00d4ff' : '#ff6464'; } } }]
});

// 策略对比柱状图
var strategyChart = echarts.init(document.getElementById('strategyChart'));
var stratNames = strategyData.map(function(d){ return d.name; });
var stratReturns = strategyData.map(function(d){ return parseFloat(d.totalReturn.toFixed(2)); });
strategyChart.setOption({
  backgroundColor: 'transparent',
  tooltip: { trigger: 'axis' },
  xAxis: { type: 'category', data: stratNames, axisLabel: { color: '#e0e0e0', rotate: 30 } },
  yAxis: { type: 'value', name: '总收益率%', axisLabel: { color: '#e0e0e0' } },
  series: [{ type: 'bar', data: stratReturns, itemStyle: { color: function(p){ return p.data >= 0 ? '#00d4ff' : '#ff6464'; } }, label: { show: true, position: 'top', formatter: function(p){ return p.data+'%'; }, color: '#e0e0e0' } }]
});

var rollingSharpeData = {{.RollingSharpeData}};
var bubbleData = {{.BubbleData}};
var pnlHistData = {{.PnLHistData}};
var holdDaysData = {{.HoldDaysHistData}};

// 滚动Sharpe曲线
var sharpeChart = echarts.init(document.getElementById('sharpeChart'));
var sharpeDates = rollingSharpeData.map(function(d){ return d.date; });
var sharpeValues = rollingSharpeData.map(function(d){ return d.sharpe; });
sharpeChart.setOption({
  backgroundColor: 'transparent',
  tooltip: { trigger: 'axis' },
  xAxis: { type: 'category', data: sharpeDates, axisLabel: { color: '#e0e0e0' } },
  yAxis: { type: 'value', name: 'Sharpe', axisLabel: { color: '#e0e0e0' } },
  series: [{
    name: '滚动Sharpe', type: 'line', data: sharpeValues,
    lineStyle: { color: '#ffd700', width: 1.5 },
    markLine: { data: [{ yAxis: 0, lineStyle: { color: '#ff6464', type: 'dashed' } }] }
  }]
});

// 策略气泡图
var bubbleChart = echarts.init(document.getElementById('bubbleChart'));
var bubbleSeriesData = bubbleData.map(function(d){
  return { value: [d.maxDD, d.totalReturn, d.sharpe, d.name], symbolSize: Math.max(Math.abs(d.sharpe) * 10, 8) };
});
bubbleChart.setOption({
  backgroundColor: 'transparent',
  tooltip: { formatter: function(p){ return p.data.value[3]+'<br>总收益:'+p.data.value[1]+'%<br>最大回撤:'+p.data.value[0]+'%<br>Sharpe:'+p.data.value[2]; } },
  xAxis: { name: '最大回撤%', axisLabel: { color: '#e0e0e0' } },
  yAxis: { name: '总收益%', axisLabel: { color: '#e0e0e0' } },
  series: [{ type: 'scatter', data: bubbleSeriesData, itemStyle: { color: '#00d4ff', opacity: 0.8 } }]
});

// 盈亏分布直方图
var pnlHistChart = echarts.init(document.getElementById('pnlHistChart'));
var pnlBins = pnlHistData.map(function(d){ return d.bin; });
var pnlCounts = pnlHistData.map(function(d){ return d.count; });
var pnlColors = pnlBins.map(function(b){ return b.indexOf('-') === 0 ? '#ff6464' : '#00d4ff'; });
pnlHistChart.setOption({
  backgroundColor: 'transparent',
  tooltip: { trigger: 'axis' },
  xAxis: { type: 'category', data: pnlBins, axisLabel: { color: '#e0e0e0', rotate: 45 } },
  yAxis: { type: 'value', name: '交易次数', axisLabel: { color: '#e0e0e0' } },
  series: [{ type: 'bar', data: pnlCounts, itemStyle: { color: function(p){ return pnlColors[p.dataIndex]; } } }]
});

// 持仓天数分布直方图
var holdDaysChart = echarts.init(document.getElementById('holdDaysChart'));
var holdBins = holdDaysData.map(function(d){ return d.bin; });
var holdCounts = holdDaysData.map(function(d){ return d.count; });
holdDaysChart.setOption({
  backgroundColor: 'transparent',
  tooltip: { trigger: 'axis' },
  xAxis: { type: 'category', data: holdBins, axisLabel: { color: '#e0e0e0', rotate: 45 } },
  yAxis: { type: 'value', name: '交易次数', axisLabel: { color: '#e0e0e0' } },
  series: [{ type: 'bar', data: holdCounts, itemStyle: { color: '#5470c6' } }]
});

window.addEventListener('resize', function(){
  equityChart.resize(); monthlyChart.resize(); tradesChart.resize(); strategyChart.resize();
  sharpeChart.resize(); bubbleChart.resize(); pnlHistChart.resize(); holdDaysChart.resize();
});
</script>
</body>
</html>`
