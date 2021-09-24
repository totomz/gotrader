const panels = [];
const synclock = {};

const uniqId = (function () {
    var i = 0;
    return function () {
        return i++;
    }
})();

function InitCandleChart(dataset, additionalSeries) {

    const candlesData = [];
    dataset.candles.x.forEach((datetime, i) => {
        candlesData[i] = [
            datetime,
            dataset.candles.open[i],
            dataset.candles.high[i],
            dataset.candles.low[i],
            dataset.candles.close[i],
        ]
    });

    const candles = {
        type: 'candlestick',
        name: 'main',
        enableMouseTracking: false,
        data: candlesData,
    }

    // create the chart
    const perdio = Highcharts.stockChart('mainchart', {
        navigation: {bindingsClassName: 'tools-container'},
        chart: {height: '40%', lacacca: 3294},
        tooltip: {enabled: false},
        stockTools: {gui: {enabled: false},},
        enableMouseTracking: false,
        xAxis: {
            type: 'datetime', crosshair: {snap: false},
            events: {setExtremes: propagateZoom}
        },
        yAxis: {title: {text: 'OHLC'}, opposite: false, crosshair: {snap: false}},
        rangeSelector: {enabled: false},
        series: [candles, ...additionalSeries],

    });
    panels.push(perdio);

    // Handle the line
    $(perdio.container).mousemove(function (event) {
        // https://stackoverflow.com/questions/15152738/highcharts-highlight-mouse-position-within-a-range
        const normalizedEvent = perdio.pointer.normalize(event);
        updateMousePosition(normalizedEvent, perdio);

        if (synclock['overmoudas'] === normalizedEvent.chartX) {
            return;
        }
        synclock['overmoudas'] = normalizedEvent.chartX

        const xVal = perdio.xAxis[0].toValue(normalizedEvent.chartX);
        const charts = Highcharts.charts;
        charts.forEach(function (chart, index) {
            if (chart.renderTo.id === perdio.renderTo.id) {
                return true
            }

            if (chart.xAxis[0].options.plotLines.length === 0) {
                chart.xAxis[0].addPlotLine({color: 'red', width: 1, value: xVal})
            } else {
                chart.xAxis[0].options.plotLines[0].value = xVal;

                const lastUpdate = synclock[`overmoudas_lastupdate_${chart.renderTo.id}`] || 0;
                if ((Date.now() - lastUpdate) > 150) {
                    chart.xAxis[0].update();
                    synclock[`overmoudas_lastupdate_${chart.renderTo.id}`] = Date.now();
                }
            }

        });

    });
}

function AddPanel(serie) {
    const divId = `panel_${uniqId()}`
    $("#mainchart").prepend(` <div class="row w-100">
            <div class="col w-100">
                <div id="${divId}"></div>
            </div>
        </div>`)

    const series = [serie];
    if (serie.range) {
        const ranges = []; // date, min, max
        serie.data.forEach((val, i) => {
            
            // .min and .max can be either a number or an array.
            // this is javascript.
            // getNumber is the javamagic function that handle this jshyt
            ranges.push([
                val[0],
                getNumber(serie.range.min, i),
                getNumber(serie.range.max, i),
            ])
        })

        series.push({
            name: 'Range',
            data: ranges,
            type: 'arearange',
            lineWidth: 0,
            linkedTo: ':previous',
            color: Highcharts.getOptions().colors[0],
            // color: 'red',
            fillOpacity: 0.2,
            zIndex: 0,
            marker: {enabled: false}
        });

    }

    const chartoptions = {
        title: false,
        chart: {height: '10%'},
        stockTools: {gui: {enabled: false},},
        marker: {enabled: false},
        enableMouseTracking: false,
        tooltip: {enabled: false},
        xAxis: {type: 'datetime', crosshair: {snap: false}, events: {setExtremes: propagateZoom}, plotLines: []},
        yAxis: {title: {text: serie.name}, opposite: false, crosshair: {snap: false}},
        plotOptions: { line: { marker: { enabled: false } } },
        series
    };
    const serieOption = serie.options || {};

    const panel = Highcharts.chart(divId, mergeDeep(chartoptions, serieOption));

    $(panel.container).mousemove(function (event) {
        const normalizedEvent = panel.pointer.normalize(event);
        updateMousePosition(normalizedEvent, panel);
    })

    panels.push(panel);
}

function formatXY(input) {
    const data = [];
    input.x.forEach((datetime, i) => {
        data[i] = [
            datetime,
            input.y[i]
        ]
    });

    return data;
}

function propagateZoom(event) {
    const myDiv = event.target.chart.renderTo.id;
    if (synclock['propagateZoom']) {
        // console.log(`propagateZoom: locked - ${myDiv} `)
        return true;
    }

    synclock['propagateZoom'] = true;

    const charts = Highcharts.charts;
    charts.forEach(function (chart, index) {
        if (chart.renderTo.id === myDiv) {
            return true
        }
        chart.xAxis[0].setExtremes(event.min, event.max, true)
    });

    synclock['propagateZoom'] = false;

}

function getNumber(interface, i) {
    if (Array.isArray(interface)) {
        return interface[i];
    }

    return interface
}

function updateMousePosition(normalizedEvent, c) {
    const xVal = c.xAxis[0].toValue(normalizedEvent.chartX);
    const yVal = c.yAxis[0].toValue(normalizedEvent.chartY);
    const xDate = new Date(xVal)
    $('#main_x').text(`${xDate.getHours()}:${xDate.getMinutes()}:${xDate.getSeconds()}`);
    $('#main_y').text(parseFloat(yVal).toFixed(4));
}

function isObject(item) {
    return (item && typeof item === 'object' && !Array.isArray(item));
}

function mergeDeep(target, source) {
    let output = Object.assign({}, target);
    if (isObject(target) && isObject(source)) {
        Object.keys(source).forEach(key => {
            if (isObject(source[key])) {
                if (!(key in target))
                    Object.assign(output, {[key]: source[key]});
                else
                    output[key] = mergeDeep(target[key], source[key]);
            } else {
                Object.assign(output, {[key]: source[key]});
            }
        });
    }
    return output;
}
