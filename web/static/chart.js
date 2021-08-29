const refreshIntervalSec = 10;
const dataDurationMinute = 270;

function init(pair, id) {
    google.charts.load('current', { packages: ['corechart'] });
    google.charts.setOnLoadCallback(() => {
        draw(pair, id);
    });
    setInterval(draw, refreshIntervalSec * 1000, pair);
    console.log("refresh interval sec:" + refreshIntervalSec);
    console.log("data duration minute:" + dataDurationMinute);
}

async function draw(pair, id) {
    try {
        const baseURL = location.protocol + '//' + location.host;

        const botInfo = await fetchBotInfo(baseURL, pair)
        if (!botInfo.markets.length) {
            return
        }

        const hasSellOrder = botInfo.statuses.sell_rate > 0;

        var values = [[
            'datetime',
            'market', { 'type': 'string', 'role': 'style' },
            'support line',
            'support line short',
            'resistance line',
            'sell volume',
            'buy volume'
        ]];
        if (hasSellOrder) {
            values[0].push('sell order')
        }

        var beforeDatetime = null;

        // サポートライン計算用
        var calcSupportLine = index => {
            const a = botInfo.statuses.support_line_slope;
            const b = botInfo.statuses.support_line_value - a * (botInfo.markets.length - 1);
            return a * index + b
        }
        var calcSupportLineShort = index => {
            const a = botInfo.statuses.support_line_short_slope;
            const b = botInfo.statuses.support_line_short_value - a * (botInfo.markets.length - 1);
            return a * index + b
        }
        // レジスタンスライン計算用
        var calcResistanceLine = index => {
            const a = botInfo.statuses.resistance_line_slope;
            const b = botInfo.statuses.resistance_line_value - a * (botInfo.markets.length - 1);
            return a * index + b
        }

        var minRate = 0.0;
        var maxRate = 0.0;
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
                point = 'point {size:7;shape-type:diamond;fill-color:#ffa500;}'
            } else if (bought) {
                point = 'point {size:7;shape-type:diamond;fill-color:#3cb371;}'
            } else if (selled) {
                point = 'point {size:7;shape-type:diamond;fill-color:#dc3545;}'
            }
            var value = [
                datetime,
                market.sell_rate, point,
                calcSupportLine(index),
                calcSupportLineShort(index),
                calcResistanceLine(index),
                market.sell_volume,
                market.buy_volume
            ];
            if (hasSellOrder) {
                value.push(botInfo.statuses.sell_rate);
            }
            values.push(value);

            beforeDatetime = datetime;

            if (index == 0) {
                maxRate = market.sell_rate;
                minRate = market.sell_rate;
            } else {
                maxRate = Math.max(maxRate, market.sell_rate);
                minRate = Math.min(minRate, market.sell_rate);
            }
        });

        if (hasSellOrder) {
            maxRate = Math.max(maxRate, botInfo.statuses.sell_rate);
            minRate = Math.min(minRate, botInfo.statuses.sell_rate);
        }

        const data = new google.visualization.arrayToDataTable(values);

        const options = {
            title: pair,
            chartArea: { top: 50, width: '80%', height: '70%' },
            hAxis: {
                title: 'Time',
                gridlines: {
                    units: {
                        days: { format: ['MM/dd'] },
                        hours: { format: ['HH:mm'] }
                    }
                }
            },
            vAxes: {
                0: { title: 'Volume' },
                1: {
                    title: 'Rate',
                    viewWindow: {
                        min: minRate * 0.99,
                        max: maxRate * 1.01
                    }
                },
            },
            backgroundColor: '#f1f8e9',
            pointSize: 1,
            seriesType: 'line',
            series: {
                0: { type: 'line', targetAxisIndex: 1, color: '#000080' },    // レート
                1: { type: 'line', targetAxisIndex: 1, color: '#ffa500' },    // サポートライン
                2: { type: 'line', targetAxisIndex: 1, color: '#87ceeb' },    // サポートライン（短期）
                3: { type: 'line', targetAxisIndex: 1, color: '#ffa500' },    // レジスタンスライン
                4: { type: 'bars', targetAxisIndex: 0, color: '#ff0000' },    // 売り出来高
                5: { type: 'bars', targetAxisIndex: 0, color: '#008000' },    // 買い出来高
                6: { type: 'line', targetAxisIndex: 1, color: '#00bfff' },    // 約定待ち売レート
            }
        };

        const chart = new google.visualization.ComboChart(document.getElementById(id));
        chart.draw(data, options);
    } catch (e) {
        console.log(e);
    }
}

async function fetchBotInfo(baseURL, pair) {
    try {
        const res = await fetch(baseURL + '/api/' + pair + "?minute=" + dataDurationMinute);
        return await res.json();
    } catch (err) {
        throw err
    }
}
