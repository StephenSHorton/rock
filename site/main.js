(function () {
  var nav = document.getElementById("nav");
  var toggle = nav.querySelector(".nav-toggle");
  var links = nav.querySelectorAll(".nav-links a");
  var vein = document.getElementById("scroll-vein");
  var lines = document.getElementById("term-lines");
  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  toggle.addEventListener("click", function () {
    var open = nav.classList.toggle("open");
    toggle.setAttribute("aria-expanded", open ? "true" : "false");
  });

  links.forEach(function (link) {
    link.addEventListener("click", function () {
      nav.classList.remove("open");
      toggle.setAttribute("aria-expanded", "false");
    });
  });

  var sections = ["install", "face", "doors", "gates", "sample", "essentials"]
    .map(function (id) { return document.getElementById(id); })
    .filter(Boolean);

  if ("IntersectionObserver" in window) {
    var current = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        links.forEach(function (link) {
          var on = link.getAttribute("href") === "#" + entry.target.id;
          if (on) link.setAttribute("aria-current", "true");
          else link.removeAttribute("aria-current");
        });
      });
    }, { rootMargin: "-40% 0px -50% 0px", threshold: 0.01 });
    sections.forEach(function (section) { current.observe(section); });
  }

  function onScroll() {
    var height = document.documentElement.scrollHeight - window.innerHeight;
    var p = height > 0 ? window.scrollY / height : 0;
    vein.style.transform = "scaleY(" + Math.max(0, Math.min(1, p)) + ")";
  }
  onScroll();
  window.addEventListener("scroll", onScroll, { passive: true });

  var tabs = Array.prototype.slice.call(document.querySelectorAll(".os-tab"));
  var panels = Array.prototype.slice.call(document.querySelectorAll(".os-panel"));

  function selectOS(os) {
    tabs.forEach(function (tab) {
      var on = tab.getAttribute("data-os") === os;
      tab.setAttribute("aria-selected", on ? "true" : "false");
      tab.tabIndex = on ? 0 : -1;
    });
    panels.forEach(function (panel) {
      var on = panel.getAttribute("data-os") === os;
      panel.classList.toggle("is-on", on);
      if (on) panel.removeAttribute("hidden");
      else panel.setAttribute("hidden", "");
    });
  }

  document.documentElement.classList.add("js");
  var platform = navigator.platform || "";
  var ua = navigator.userAgent || "";
  var initial = "linux";
  if (/Win/.test(platform) || /Windows/.test(ua)) initial = "windows";
  else if (/Mac/.test(platform) || /Mac OS/.test(ua)) initial = "mac";
  selectOS(initial);

  tabs.forEach(function (tab) {
    tab.addEventListener("click", function () {
      selectOS(tab.getAttribute("data-os"));
    });
  });

  document.querySelectorAll(".copy").forEach(function (button) {
    button.addEventListener("click", function () {
      var node = document.getElementById(button.getAttribute("data-copy"));
      var text = node ? node.textContent : "";
      var done = function () {
        var previous = button.textContent;
        button.textContent = "Copied";
        window.setTimeout(function () { button.textContent = previous; }, 1400);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, function () { fallbackCopy(text, done); });
      } else {
        fallbackCopy(text, done);
      }
    });
  });

  function fallbackCopy(text, done) {
    var area = document.createElement("textarea");
    area.value = text;
    document.body.appendChild(area);
    area.select();
    try { document.execCommand("copy"); } catch (err) { /* the command stays selectable */ }
    document.body.removeChild(area);
    done();
  }

  if (reduce) return;

  var items = Array.prototype.slice.call(lines.children);
  lines.classList.add("is-armed");
  items.forEach(function (item, i) {
    window.setTimeout(function () { item.classList.add("on"); }, 500 + i * 420);
  });
})();
