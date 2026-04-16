(function () {
  var canvas = document.getElementById('firingChart');
  if (!canvas) return;

  var firingId = canvas.dataset.firingId;

  fetch('/api/firings/' + firingId + '/readings')
    .then(function (res) { return res.json(); })
    .then(function (data) {
      var readings = data.readings || [];
      var labels = readings.map(function (r) { return r.elapsed_minutes; });
      var temps  = readings.map(function (r) { return r.temperature; });
      var pointLabels = readings.map(function (r) {
        return (r.gas_setting || '') + (r.flue_setting ? ' / ' + r.flue_setting : '');
      });

      new Chart(canvas, {
        type: 'line',
        data: {
          labels: labels,
          datasets: [{
            label: 'Temperature (\u00b0C)',
            data: temps,
            borderColor: '#c0392b',
            backgroundColor: 'rgba(192,57,43,0.1)',
            pointRadius: 4,
            tension: 0.2
          }]
        },
        options: {
          responsive: true,
          plugins: {
            tooltip: {
              callbacks: {
                afterLabel: function (ctx) {
                  return pointLabels[ctx.dataIndex] || '';
                }
              }
            }
          },
          scales: {
            x: { title: { display: true, text: 'Elapsed (min)' } },
            y: { title: { display: true, text: 'Temperature (\u00b0C)' } }
          }
        }
      });
    })
    .catch(function (err) { console.error('firing chart error', err); });
}());
