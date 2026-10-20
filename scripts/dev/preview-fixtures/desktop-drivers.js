// * Datos de ejemplo de los drivers del USB para capturar la página de drivers de Rebootera Desktop (PREVIEW_FIXTURE=desktop-drivers).
(function () {
  const base = window.fetch.bind(window);
  const pack = function (dir, inf, provider, cls, signature) {
    return { dir: dir, inf: dir + "/" + inf, provider: provider, class: cls, version: "12.19.0", date: "2024-03-01", signature: signature };
  };
  const body = {
    available: true,
    root: "E:\\",
    packages: [
      pack("intel-red", "e1d.inf", "Intel", "Net", "present"),
      pack("intel-red", "e1r.inf", "Intel", "Net", "present"),
      pack("intel-rst", "iaStorVD.inf", "Intel", "SCSIAdapter", "present"),
      pack("realtek-wifi", "rtwlane.inf", "Realtek", "Net", "missing"),
    ],
  };
  window.fetch = function (input, init) {
    const url = typeof input === "string" ? input : input.url;
    if (/\/api\/desktop\/drivers$/.test(url) && (!init || !init.method || init.method === "GET")) {
      return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    }
    return base(input, init);
  };
})();
