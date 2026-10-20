// * Datos de ejemplo del gestor de particiones para scripts/dev/preview-ui.sh (PREVIEW_FIXTURE=partitions).
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

// * Datos extra para capturar el asistente de instalación y la página de drivers: biblioteca, USB, dispositivos y casado de drivers.
(function () {
  const base = window.fetch.bind(window);
  const json = function (body) {
    return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
  };
  const pack = function (sourceId, inf, provider, cls) {
    return { sourceId: sourceId, dir: "intel", inf: inf, provider: provider, class: cls, version: "12.19.0", date: "2024-03-01", catalog: "x.cat", signature: "present" };
  };
  const device = function (address, name, vendor, category, driver, critical) {
    return { bus: "pci", address: address, vendorId: "8086", deviceId: "15bb", vendor: vendor, name: name, category: category, className: name, driver: driver, hardwareIds: ["PCI\VEN_8086&DEV_15BB"], critical: critical };
  };
  const devices = [
    device("0000:00:1f.6", "Ethernet Connection I219-V", "Intel", "network", "e1000e", true),
    device("0000:02:00.0", "RTL8852AE Wi-Fi 6", "Realtek", "wireless", "", true),
    device("0000:03:00.0", "NVMe SSD Controller", "Samsung", "storage", "nvme", true),
    device("0000:00:02.0", "UHD Graphics 630", "Intel", "display", "i915", false),
    device("0000:00:1f.3", "Cannon Lake Audio", "Intel", "audio", "snd_hda_intel", false),
    device("0000:00:14.0", "USB 3.1 xHCI", "Intel", "usb-controller", "xhci_hcd", false),
  ];
  const plan = {
    available: true,
    packages: 312,
    sources: [
      { id: "usb", kind: "usb", packages: 12 },
      { id: "win", kind: "windows", device: "nvme0n1p2", packages: 300 },
    ],
    matches: [
      { address: "0000:00:1f.6", name: "Ethernet Connection I219-V", vendor: "Intel", category: "network", critical: true, hardwareId: "PCI\VEN_8086&DEV_15BB", specific: true, description: "", package: pack("usb", "e1d.inf", "Intel", "Net") },
      { address: "0000:03:00.0", name: "NVMe SSD Controller", vendor: "Samsung", category: "storage", critical: true, hardwareId: "PCI\VEN_144D&DEV_A808", specific: true, description: "", package: pack("win", "stornvme.inf", "Microsoft", "SCSIAdapter") },
    ],
    missing: [
      { address: "0000:02:00.0", name: "RTL8852AE Wi-Fi 6", vendor: "Realtek", category: "wireless", critical: true, hardwareId: "PCI\VEN_10EC&DEV_8852", specific: true, description: "", package: null },
    ],
  };
  const library = [
    { path: "isos/windows/Win11_24H2_Spanish_x64.iso", name: "Win11_24H2_Spanish_x64.iso", category: "windows", kind: "iso", sizeBytes: 5400000000, modified: "", sha256: "", state: "verified", source: "catalog", progress: 1, bootStatus: "verified", title: "Windows 11 24H2" },
    { path: "isos/linux/debian-13.1.0-amd64-netinst.iso", name: "debian-13.1.0-amd64-netinst.iso", category: "linux", kind: "iso", sizeBytes: 700000000, modified: "", sha256: "", state: "verified", source: "debian", progress: 1, bootStatus: "verified", title: "Debian 13" },
  ];
  window.fetch = function (input, init) {
    const url = typeof input === "string" ? input : input.url;
    const get = !init || !init.method || init.method === "GET";
    if (get && /\/api\/vault$/.test(url)) {
      return json({ available: true, usedBytes: 9000000000, freeBytes: 48000000000, nextBoot: "" });
    }
    if (get && /\/api\/library$/.test(url)) {
      return json(library);
    }
    if (get && /\/api\/hardware$/.test(url)) {
      return json(devices);
    }
    if (get && /\/api\/drivers\/plan$/.test(url)) {
      return json(plan);
    }
    return base(input, init);
  };
  // * Hace creer a la página que corre dentro del kiosco (para ver el botón Navegador).
  try {
    Object.defineProperty(window, "webkit", { value: { messageHandlers: { rebootera: { postMessage: function () {} } } }, configurable: true });
  } catch (e) {}
})();

// * Medición y plan de las operaciones de particiones (los diálogos de Discos y de "Hacer sitio").
(function () {
  const base = window.fetch.bind(window);
  const json = function (body) {
    return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
  };
  window.fetch = function (input, init) {
    const url = typeof input === "string" ? input : input.url;
    if (url.indexOf("/api/partitions/info") >= 0) {
      return json({ minBytes: 150000000000 });
    }
    if (url.indexOf("/api/partitions/plan") >= 0) {
      return json({ steps: [{ kind: "check", part: 2 }, { kind: "shrink-fs", part: 2 }, { kind: "table", part: 2 }], destructive: false, movesBytes: 0, affected: [2], confirm: "CHANGE nvme0n1" });
    }
    return base(input, init);
  };
})();
