var s = new goog.appengine.Channel(tok).open();
s.onmessage = function(link) {
  console.log(link);
  req = new XMLHttpRequest();
  req.onload = function() {
    console.log('got ' + link);
    imgs = JSON.parse(req.responseText);
    console.log(imgs);

    var img = document.createElement('img');
    img.id = 'img';
    img.src = imgs[0];

    var slider = document.createElement('input');
    slider.type = 'range';
    slider.min = '0';
    slider.max = ''+total-1;
    slider.onchange = function() {
      var src = imgs[parseInt(slider.value)];
      if (src != undefined) {
        img.src = src;
      }
    };
    slider.value = '0';
    slider.style.display = 'block';
    slider.style.width = img.width;

    document.body.innerHTML = '';
    document.body.appendChild(img);
    document.body.appendChild(slider);
  };
  req.open('GET', link);
  req.send();
};
