function Init(){
    const trace1 = {
      type: 'bar',
      x: [1, 2, 3, 4],
      y: [5, 10, 2, 8],
      marker: {
          color: '#C8A2C8',
          line: {
              width: 2.5
          }
      }
    };
    
    const data = [ trace1 ];
    
    const layout = { 
      title: 'Responsive to window\'s size!',
      font: {size: 18}
    };
    
    const config = {responsive: true}
    
    Plotly.newPlot('myDiv', data, layout, config );

}