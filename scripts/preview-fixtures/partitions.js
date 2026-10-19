// * Datos de ejemplo del gestor de particiones para scripts/preview-ui.sh (PREVIEW_FIXTURE=partitions).
// * Solo se usa para dibujar capturas de la interfaz; no toca ningún disco.
(function () {
  const MIB = 2048;
  function layout(name, parts) {
    let start = 2048;
    const segments = parts.map(function (p) {
      const sectors = p.mib * MIB;
      const segment = { number: p.n, name: p.n ? name + "p" + p.n : undefined, start: start, sectors: sectors, fsType: p.fs, label: p.label, mounted: false, usedBytes: (p.used || 0) * 1048576, resizable: p.fs === "ntfs" || p.fs === "ext4" };
      start += sectors;
      return segment;
    });
    return { disk: name, table: "gpt", totalSectors: start + 2048, limit: start, segments: segments };
  }
  const disks = [
    { name: "nvme0n1", model: "Samsung SSD 980", vendor: "", transport: "nvme", role: "internal", sizeBytes: 512110190592,
      layout: layout("nvme0n1", [
        { n: 1, mib: 260, fs: "vfat", label: "EFI", used: 60 },
        { n: 2, mib: 204800, fs: "ntfs", label: "Windows", used: 133000 },
        { n: 3, mib: 1024, fs: "ntfs", label: "Recuperación", used: 700 },
        { n: 0, mib: 51200 },
        { n: 4, mib: 102400, fs: "ext4", label: "Linux", used: 22000 },
        { n: 5, mib: 81920, fs: "exfat", label: "Datos", used: 61000 },
      ]) },
    { name: "sdb", model: "Ultra USB 3.0", vendor: "SanDisk", transport: "usb", role: "removable", sizeBytes: 61530439680,
      layout: layout("sdb", [
        { n: 1, mib: 57000, fs: "exfat", label: "REBOOTERA", used: 9000 },
        { n: 2, mib: 32, fs: "vfat", label: "VTOYEFI", used: 20 },
      ]) },
  ];
  const original = window.fetch.bind(window);
  window.fetch = function (input, init) {
    const url = typeof input === "string" ? input : input.url;
    const json = function (body) {
      return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    };
    if (url.indexOf("/api/partitions/status") >= 0) {
      return json({ running: false, done: false, disk: "", step: 0, total: 0, kind: "", part: 0, progress: 0, seconds: 0 });
    }
    if (url.indexOf("/api/partitions") >= 0 && (!init || !init.method || init.method === "GET")) {
      return json(disks);
    }
    return original(input, init);
  };
})();
