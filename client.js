var imgs = [];

var s = new goog.appengine.Channel(tok).open();
s.onmessage = function(m) {
  var d = JSON.parse(m.data);

  // Init imgs to undef-filled arr
  if (imgs.length < d.tf - 1) {
    imgs[d.tf - 1] = undefined;
  }

  // Init parts to undef-filled arr
  if (imgs[d.f] == undefined) {
    imgs[d.f] = [];
    imgs[d.f][d.tp - 1] = undefined;
  }
  imgs[d.f][d.p] = d.d;

  if (noneUndefined(imgs[d.f])) {
    // This frame is done, reassemble its data
    imgs[d.f] = imgs[d.f].join('');
    document.body.innerHTML = 'Loaded ' + d.f + ' of ' + d.tf;

    if (noneUndefined(imgs)) {
      // All frames are done, show the image
      show(d.tf);
    }
  }
};

var noneUndefined = function(arr) {
  for (var i = 0; i < arr.length; i++) {
    if (arr[i] == undefined) {
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
