var imgs = [];

var s = new goog.appengine.Channel(tok).open();
s.onmessage = function(m) {
  var d = JSON.parse(m.data);
  imgs[d.I] = d.F;
  document.body.innerHTML = 'Loaded ' + d.I + ' of ' + d.L;
  if (ready(d.L)) {
    show(d.L);
  }
};

var ready = function(total) {
  for (var i = 0; i < total; i++) {
    if (imgs[i] == undefined) {
      return false;
    }
  }
  return true;
};

var show = function(total) {
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
