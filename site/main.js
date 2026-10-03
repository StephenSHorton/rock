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

  var sections = ["face", "doors", "gates", "sample", "essentials"]
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

  if (reduce) return;

  var items = Array.prototype.slice.call(lines.children);
  lines.classList.add("is-armed");
  items.forEach(function (item, i) {
    window.setTimeout(function () { item.classList.add("on"); }, 500 + i * 420);
  });
})();
