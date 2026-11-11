// Lider Otomarket — küçük etkileşimler. Favoriler tarayıcıda (localStorage) saklanır, üyelik yok.
(function () {
  "use strict";

  var KEY = "lom.favoriler";

  // ---------- Favori deposu ----------
  function readFavs() {
    try {
      var v = JSON.parse(localStorage.getItem(KEY) || "[]");
      return Array.isArray(v) ? v.filter(function (x) { return typeof x === "string" && x; }) : [];
    } catch (e) { return []; }
  }
  function writeFavs(list) {
    try { localStorage.setItem(KEY, JSON.stringify(list)); } catch (e) { /* gizli mod vb. */ }
  }
  var favs = readFavs();
  function isFav(id) { return favs.indexOf(id) !== -1; }
  function setFav(id, on) {
    var i = favs.indexOf(id);
    if (on && i === -1) favs.unshift(id);
    if (!on && i !== -1) favs.splice(i, 1);
    writeFavs(favs);
    render();
  }

  function render() {
    document.querySelectorAll("[data-fav]").forEach(function (btn) {
      var id = btn.getAttribute("data-fav");
      var on = isFav(id);
      var ad = btn.getAttribute("data-ad") || "Ürün";
      btn.setAttribute("aria-pressed", on ? "true" : "false");
      if (btn.hasAttribute("data-etiketli")) {
        btn.setAttribute("aria-label", on ? "Favorilerden çıkar" : "Favorilere ekle");
      } else {
        btn.setAttribute("aria-label", on ? ad + " favorilerden çıkar" : ad + " favorilere ekle");
      }
      var metin = btn.querySelector("[data-fav-metin]");
      if (metin) metin.textContent = on ? "Favorilerde" : "Favorilere ekle";
    });
    document.querySelectorAll("[data-fav-sayac]").forEach(function (el) {
      el.textContent = String(favs.length);
      el.hidden = favs.length === 0;
    });
  }

  document.addEventListener("click", function (e) {
    var btn = e.target.closest("[data-fav]");
    if (btn) {
      e.preventDefault();
      var id = btn.getAttribute("data-fav");
      setFav(id, !isFav(id));
      return;
    }
    var kaldir = e.target.closest("[data-fav-kaldir]");
    if (kaldir) {
      e.preventDefault();
      setFav(kaldir.getAttribute("data-fav-kaldir"), false);
      var row = kaldir.closest(".fav-satir");
      if (row) row.remove();
      updateFavPage();
    }
  });

  // ---------- Favoriler sayfası ----------
  var favList = document.querySelector("[data-favori-liste]");
  function updateFavPage() {
    if (!favList) return;
    var count = favList.querySelectorAll(".fav-satir").length;
    var empty = document.querySelector("[data-favori-bos]");
    if (empty) empty.hidden = count > 0;
    document.querySelectorAll("[data-favori-sayi]").forEach(function (el) {
      el.textContent = count + " ürün";
    });
  }
  if (favList) {
    if (favs.length === 0) {
      updateFavPage();
    } else {
      fetch("/parca/favoriler?ids=" + encodeURIComponent(favs.join(",")), { headers: { Accept: "text/html" } })
        .then(function (r) { return r.ok ? r.text() : ""; })
        .then(function (html) {
          favList.innerHTML = html;
          // Sitede artık olmayan ürünleri listeden temizle.
          var present = Array.prototype.map.call(favList.querySelectorAll(".fav-satir"), function (el) {
            return el.getAttribute("data-id");
          });
          if (html && present.length !== favs.length) {
            favs = favs.filter(function (id) { return present.indexOf(id) !== -1; });
            writeFavs(favs);
            render();
          }
          updateFavPage();
        })
        .catch(updateFavPage);
    }
  }

  // ---------- Açılır menüler (masaüstü "Kategoriler") ----------
  function closeDropdowns(except) {
    document.querySelectorAll("[data-acilir][aria-expanded='true']").forEach(function (b) {
      if (b === except) return;
      b.setAttribute("aria-expanded", "false");
      var t = document.getElementById(b.getAttribute("aria-controls"));
      if (t) t.hidden = true;
    });
  }
  document.querySelectorAll("[data-acilir]").forEach(function (b) {
    b.addEventListener("click", function (e) {
      e.stopPropagation();
      var open = b.getAttribute("aria-expanded") === "true";
      closeDropdowns(b);
      b.setAttribute("aria-expanded", open ? "false" : "true");
      var t = document.getElementById(b.getAttribute("aria-controls"));
      if (t) t.hidden = open;
    });
  });
  document.addEventListener("click", function (e) {
    if (!e.target.closest(".kat-acilir")) closeDropdowns(null);
  });

  // ---------- Akordiyon (mobil menü kategorileri) ----------
  document.querySelectorAll("[data-akordiyon]").forEach(function (b) {
    b.addEventListener("click", function () {
      var open = b.getAttribute("aria-expanded") === "true";
      b.setAttribute("aria-expanded", open ? "false" : "true");
      var t = document.getElementById(b.getAttribute("aria-controls"));
      if (t) t.hidden = open;
    });
  });

  // ---------- Tam ekran katmanlar (mobil menü, filtre paneli) ----------
  var lastOpener = null;
  function openLayer(id, opener) {
    var el = document.getElementById(id);
    if (!el) return;
    lastOpener = opener || null;
    el.hidden = false;
    document.documentElement.classList.add("menu-acik");
    var focusable = el.querySelector("[data-ilk-odak]") || el.querySelector("button, a, input, select");
    if (focusable) focusable.focus();
  }
  function closeLayer(el) {
    el.hidden = true;
    if (!document.querySelector("[data-katman]:not([hidden])")) {
      document.documentElement.classList.remove("menu-acik");
    }
    if (lastOpener) lastOpener.focus();
  }
  document.addEventListener("click", function (e) {
    var opener = e.target.closest("[data-katman-ac]");
    if (opener) {
      e.preventDefault();
      openLayer(opener.getAttribute("data-katman-ac"), opener);
      return;
    }
    var link = e.target.closest("[data-katman] a[href]");
    if (link) {
      var owner = link.closest("[data-katman]");
      lastOpener = null;
      if (owner) closeLayer(owner);
      return;
    }
    var closer = e.target.closest("[data-katman-kapat]");
    if (closer) {
      e.preventDefault();
      var layer = closer.closest("[data-katman]");
      if (layer) closeLayer(layer);
    }
  });
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    closeDropdowns(null);
    document.querySelectorAll("[data-katman]:not([hidden])").forEach(closeLayer);
  });

  // ---------- Filtre formları ----------
  document.querySelectorAll("[data-otomatik]").forEach(function (el) {
    el.addEventListener("change", function () {
      if (el.form) el.form.submit();
    });
  });
  document.querySelectorAll("[data-panel-secim]").forEach(function (el) {
    el.addEventListener("change", function () {
      var b = document.querySelector("[data-panel-goster]");
      if (b) b.textContent = "Sonuçları göster";
    });
  });
  document.querySelectorAll(".js-gizle").forEach(function (el) { el.hidden = true; });

  // ---------- Ürün galerisi ----------
  document.querySelectorAll("[data-galeri]").forEach(function (g) {
    var data;
    try { data = JSON.parse(g.getAttribute("data-galeri")); } catch (e) { return; }
    if (!data || data.length < 2) return;
    var img = g.querySelector("[data-galeri-gorsel]");
    var label = g.getAttribute("data-ad") || "Ürün";
    var idx = 0;
    function show(i) {
      idx = (i + data.length) % data.length;
      if (img) {
        img.src = data[idx];
        img.alt = label + " — görsel " + (idx + 1) + " / " + data.length;
      }
      g.querySelectorAll("[data-gorsel-sec]").forEach(function (b) {
        b.setAttribute("aria-pressed", Number(b.getAttribute("data-gorsel-sec")) === idx ? "true" : "false");
      });
    }
    g.addEventListener("click", function (e) {
      var pick = e.target.closest("[data-gorsel-sec]");
      if (pick) { show(Number(pick.getAttribute("data-gorsel-sec"))); return; }
      if (e.target.closest("[data-galeri-onceki]")) { show(idx - 1); return; }
      if (e.target.closest("[data-galeri-sonraki]")) { show(idx + 1); }
    });
    // Mobilde kaydırma ile geçiş.
    var startX = null;
    var area = g.querySelector(".galeri-ana");
    if (area) {
      area.addEventListener("touchstart", function (e) { startX = e.touches[0].clientX; }, { passive: true });
      area.addEventListener("touchend", function (e) {
        if (startX === null) return;
        var dx = e.changedTouches[0].clientX - startX;
        if (Math.abs(dx) > 40) show(idx + (dx < 0 ? 1 : -1));
        startX = null;
      });
    }
  });

  // ---------- Mobil arama kutusu (üst bardaki büyüteç) ----------
  document.querySelectorAll("[data-arama-ac]").forEach(function (b) {
    b.addEventListener("click", function () {
      var panel = document.getElementById(b.getAttribute("aria-controls"));
      if (!panel) return;
      var open = panel.hidden;
      panel.hidden = !open;
      b.setAttribute("aria-expanded", open ? "true" : "false");
      if (open) {
        var input = panel.querySelector("input");
        if (input) input.focus();
      }
    });
  });

  // ---------- Yazarken arama sonuçları ----------
  document.querySelectorAll("[data-canli-arama]").forEach(function (form) {
    var input = form.querySelector("input[name=q]");
    var box = document.getElementById(input && input.getAttribute("aria-controls"));
    if (!input || !box) return;
    var timer = 0, seq = 0, cache = {};
    function hide() { box.hidden = true; input.setAttribute("aria-expanded", "false"); }
    function show(html) { box.innerHTML = html; box.hidden = !html; input.setAttribute("aria-expanded", html ? "true" : "false"); }
    function run() {
      var q = input.value.trim();
      if (q.length < 2) { hide(); return; }
      if (cache[q] !== undefined) { show(cache[q]); return; }
      var my = ++seq;
      fetch("/parca/arama?q=" + encodeURIComponent(q), { headers: { Accept: "text/html" } })
        .then(function (r) { return r.ok ? r.text() : ""; })
        .then(function (html) {
          cache[q] = html.trim();
          if (my === seq && document.activeElement === input) show(cache[q]);
        })
        .catch(hide);
    }
    input.addEventListener("input", function () { clearTimeout(timer); timer = setTimeout(run, 200); });
    input.addEventListener("focus", function () { if (input.value.trim().length >= 2) run(); });
    input.addEventListener("keydown", function (e) {
      if (e.key === "Escape") { hide(); return; }
      if (e.key === "ArrowDown" && !box.hidden) {
        var first = box.querySelector("a");
        if (first) { e.preventDefault(); first.focus(); }
      }
    });
    box.addEventListener("keydown", function (e) {
      var links = Array.prototype.slice.call(box.querySelectorAll("a"));
      var i = links.indexOf(document.activeElement);
      if (e.key === "ArrowDown" && i < links.length - 1) { e.preventDefault(); links[i + 1].focus(); }
      if (e.key === "ArrowUp") { e.preventDefault(); if (i > 0) links[i - 1].focus(); else input.focus(); }
      if (e.key === "Escape") { hide(); input.focus(); }
    });
    form.addEventListener("submit", function (e) {
      if (!input.value.trim()) { e.preventDefault(); input.focus(); }
    });
    document.addEventListener("click", function (e) {
      if (!form.contains(e.target) && !box.contains(e.target)) hide();
    });
  });

  // ---------- Kayan ürün şeritleri (oklar + otomatik kayma) ----------
  var reduceMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  document.querySelectorAll("[data-karusel]").forEach(function (k) {
    var ray = k.querySelector(".karusel-ray");
    var prev = k.querySelector("[data-karusel-onceki]");
    var next = k.querySelector("[data-karusel-sonraki]");
    if (!ray || !prev || !next) return;
    function step() {
      var item = ray.querySelector(".karusel-oge");
      if (!item) return 0;
      var gap = parseFloat(getComputedStyle(ray).columnGap) || 0;
      return item.getBoundingClientRect().width + gap;
    }
    function go(dir) {
      var max = ray.scrollWidth - ray.clientWidth;
      if (dir > 0 && ray.scrollLeft >= max - 4) ray.scrollTo({ left: 0 });
      else if (dir < 0 && ray.scrollLeft <= 4) ray.scrollTo({ left: max });
      else ray.scrollBy({ left: dir * step() });
    }
    function layout() {
      var over = ray.scrollWidth > ray.clientWidth + 4;
      prev.hidden = next.hidden = !over;
      var img = ray.querySelector(".kart-gorsel");
      if (img && over) {
        var top = Math.max(0, img.offsetHeight / 2 - prev.offsetHeight / 2 + 4);
        prev.style.top = next.style.top = top + "px";
      }
    }
    var pausedUntil = 0, hover = false, focus = false;
    prev.addEventListener("click", function () { pausedUntil = Date.now() + 8000; go(-1); });
    next.addEventListener("click", function () { pausedUntil = Date.now() + 8000; go(1); });
    k.addEventListener("mouseenter", function () { hover = true; });
    k.addEventListener("mouseleave", function () { hover = false; });
    k.addEventListener("focusin", function () { focus = true; });
    k.addEventListener("focusout", function () { focus = false; });
    ray.addEventListener("touchstart", function () { pausedUntil = Date.now() + 8000; }, { passive: true });
    window.addEventListener("resize", layout);
    layout();
    var ms = Number(k.getAttribute("data-sure")) || 0;
    if (ms > 0 && !reduceMotion) {
      setInterval(function () {
        if (hover || focus || document.hidden || Date.now() < pausedUntil) return;
        if (ray.scrollWidth <= ray.clientWidth + 4) return;
        go(1);
      }, ms);
    }
  });

  // ---------- Ana sayfa vitrin slaytları (yalnızca elle) ----------
  document.querySelectorAll("[data-vitrin]").forEach(function (v) {
    var ray = v.querySelector(".vitrin-ray");
    var slides = v.querySelectorAll(".vitrin-slayt");
    var dots = v.querySelectorAll("[data-vitrin-git]");
    if (!ray || slides.length < 2) return;
    function current() { return Math.round(ray.scrollLeft / Math.max(1, ray.clientWidth)); }
    function show(i) {
      i = (i + slides.length) % slides.length;
      ray.scrollTo({ left: i * ray.clientWidth });
    }
    function sync() {
      var i = current();
      slides.forEach(function (s, j) {
        if (j === i) s.removeAttribute("aria-hidden"); else s.setAttribute("aria-hidden", "true");
        s.querySelectorAll("a").forEach(function (a) { if (j === i) a.removeAttribute("tabindex"); else a.setAttribute("tabindex", "-1"); });
      });
      dots.forEach(function (d, j) { if (j === i) d.setAttribute("aria-current", "true"); else d.removeAttribute("aria-current"); });
    }
    var raf = 0;
    ray.addEventListener("scroll", function () { cancelAnimationFrame(raf); raf = requestAnimationFrame(sync); });
    var p = v.querySelector("[data-vitrin-onceki]"), n = v.querySelector("[data-vitrin-sonraki]");
    if (p) p.addEventListener("click", function () { show(current() - 1); });
    if (n) n.addEventListener("click", function () { show(current() + 1); });
    dots.forEach(function (d) { d.addEventListener("click", function () { show(Number(d.getAttribute("data-vitrin-git"))); }); });
  });

  render();
})();
