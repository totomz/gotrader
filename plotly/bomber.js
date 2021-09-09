function uniqId(name) {
    return `${name}_${Math.round(new Date().getTime() + (Math.random() * 100))}`;
}

async function addCharts() {

    const dataset = await $.get(DATASET_URL);
    dataset_golobal = dataset;
    console.log("downloaded dataset", dataset);
    
    Object.keys(dataset).forEach(k => {
        $("#serieList").append(new Option(k, k));
    });


    // Put your signals here
    const charts = [{
        trace: [{
            name: 'Volume',
            type: 'bar',
            x: dataset_golobal.candles.x,
            y: dataset_golobal.candles.volume,
        }],
        layout,
        options
    }, {
        trace: [{
            name: 'Cash',
            type: 'scatter',
            x: dataset_golobal.cash.x,
            y: dataset_golobal.cash.y,
        }],
        layout,
        options
    }, {
        trace: [{
            masterZoom: true,
            name: 'Candles',
            type: 'candlestick',

            x: dataset.candles.x,
            close: dataset.candles.close,
            high: dataset.candles.high,
            low: dataset.candles.low,
            open: dataset.candles.open,

            decreasing: {line: {color: '#7F7F7F'}},
            increasing: {line: {color: '#17BECF'}},
            line: {color: 'rgba(31,119,180,1)'},

            xaxis: 'x',
            yaxis: 'y'
        }],
        layout: {...layout, ...{height: 600}},
        options: {displayModeBar: true}
    }];

    charts.forEach(c => {
        const divId = uniqId(`chart_${c.trace[0].name}`)
        $("#chartPanel").append(`<div id="${divId}">`);
        Plotly.newPlot(divId, c.trace, c.layout, c.options);

        if (divId.startsWith('chart_Candles_')) {
            const div = document.getElementById(divId);
            div.on("plotly_relayout", function (ed) {
                relayout(ed);
            });
        } else {
            $(`#${divId}`).addClass("chartSlave")
        }
    });
}

function relayout(ed) {
    const divs = document.getElementsByClassName('chartSlave');
    for (let i = 0; i< divs.length; i++) {
        const div = divs[i]
        var update = {
            'xaxis.range[0]': ed["xaxis.range[0]"],
            'xaxis.range[1]': ed["xaxis.range[1]"],
            'xaxis.autorange': ed["xaxis.autorange"],
        };
        Plotly.relayout(div, update);
    }
}

