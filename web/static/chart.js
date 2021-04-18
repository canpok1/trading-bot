const refreshIntervalSec = 10;
const dataDurationMinute = 270;

function init(pair) {
    google.charts.load('current', { packages: ['corechart', 'line'] });
    google.charts.setOnLoadCallback(() => {
        draw(pair);
    });
    setInterval(draw, refreshIntervalSec * 1000, pair);
    console.log("refresh interval sec:" + refreshIntervalSec);
    console.log("data duration minute:" + dataDurationMinute);
}

async function draw(pair) {
    console.log("draw[" + pair + "]");

    const baseURL = location.protocol + '//' + location.host;

    const botInfo = await fetchBotInfo(baseURL, pair)

    const hasSellOrder = botInfo.statuses.sell_rate > 0;

    var values = [
        ['datetime', 'market', { 'type': 'string', 'role': 'style' }, 'support line']
    ];
    if (hasSellOrder) {
        values[0].push('sell order')
    }

    var beforeDatetime = null;

    // サポートライン計算用
    const a = botInfo.statuses.support_line_slope;
    const b = botInfo.statuses.support_line_value - a * (botInfo.markets.length - 1);

    botInfo.markets.forEach((market, index) => {
        var datetime = new Date(market.datetime);

        var bought = false;
        var selled = false;
        botInfo.events.forEach(e => {
            const eventDatetime = new Date(e.datetime)
            var matched = false;
            if (beforeDatetime != null) {
                matched = beforeDatetime < eventDatetime && eventDatetime <= datetime;
            } else {
                matched = eventDatetime <= datetime;
            }

            if (matched && e.type == 0) {
                bought = true;
            }
            if (matched && e.type == 1) {
                selled = true;
            }
        });

        var point = null;
        if (bought && selled) {
            point = 'point {size:7;shape-type:diamond;fill-color:#ffc107;}'
        } else if (bought) {
            point = 'point {size:7;shape-type:diamond;fill-color:#3cb371;}'
        } else if (selled) {
            point = 'point {size:7;shape-type:diamond;fill-color:#dc3545;}'
        }
        var value = [datetime, market.sell_rate, point, a * index + b];
        if (hasSellOrder) {
            value.push(botInfo.statuses.sell_rate);
        }
        values.push(value);

        beforeDatetime = datetime;
    });

    const data = new google.visualization.arrayToDataTable(values);

    const options = {
        hAxis: {
            title: 'Time',
            gridlines: {
                units: {
                    days: { format: ['MM/dd'] },
                    hours: { format: ['HH:mm'] }
                }
            }
        },
        vAxis: {
            title: 'Rate'
        },
        backgroundColor: '#f1f8e9',
        pointSize: 1,
    };

    const chart = new google.visualization.LineChart(document.getElementById('chart_div'));
    chart.draw(data, options);
}

async function fetchBotInfo(baseURL, pair) {
    try {
        const res = await fetch(baseURL + '/api/' + pair + "?minute=" + dataDurationMinute);
        return await res.json();
    } catch (err) {
        throw err
    }
}
