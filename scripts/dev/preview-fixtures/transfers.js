// * Datos de ejemplo de una descarga en marcha para scripts/dev/preview-ui.sh (PREVIEW_FIXTURE=transfers): la cola de
// * transferencias (una en marcha, otra terminada, otra fallida) y la creación de un USB descargando Rebootera OS.
// * Solo se usa para dibujar capturas de la interfaz; no descarga nada.
(function () {
  const jobs = [
    { id: "a1", kind: "download", name: "Windows 11 Home / Pro", catalogId: "windows-11", fileName: "es-es_windows_11_consumer_editions_x64_dvd.iso", category: "windows", totalBytes: 5800000000, doneBytes: 1740000000, speedBps: 24000000, status: "running" },
    { id: "a2", kind: "download", name: "Debian 13 netinst", catalogId: "debian-13-netinst", fileName: "debian-13.7.0-amd64-netinst.iso", category: "linux", totalBytes: 800000000, doneBytes: 800000000, speedBps: 0, status: "done" },
    { id: "a3", kind: "download", name: "Fedora Workstation 44", catalogId: "fedora-44-workstation", fileName: "Fedora-Workstation-Live-44-1.7.x86_64.iso", category: "linux", totalBytes: 2900000000, doneBytes: 100000000, speedBps: 0, status: "failed", errorCode: "failed" },
  ];
  const original = window.fetch.bind(window);
  window.fetch = function (input, init) {
    const url = typeof input === "string" ? input : input.url;
    const method = (init && init.method) || "GET";
    const json = function (body) {
      return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    };
    if (method === "GET" && url.indexOf("/api/transfers") >= 0 && url.indexOf("settings") < 0) {
      return json(jobs);
    }
    if (method === "GET" && url.indexOf("/api/desktop/create") >= 0) {
      return json({ running: true, phase: "download", progress: 0.13, done: false, paused: false });
    }
    return original(input, init);
  };
})();
