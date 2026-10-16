// * Gráficas pequeñas en SVG para los widgets del resumen: línea con área
// * (temperaturas) y barras (tráfico de red). Se dibujan con datos reales del
// * historial de state.ts; si no hay datos suficientes se dibuja una base vacía.

const SVG_NS = "http://www.w3.org/2000/svg";

/** Crea un elemento SVG con atributos. */
function svgEl(tag: string, attrs: Record<string, string>): SVGElement {
  const node = document.createElementNS(SVG_NS, tag);
  for (const name of Object.keys(attrs)) {
    node.setAttribute(name, attrs[name]);
  }
  return node;
}

/** Dibuja una línea con área sombreada bajo ella. */
export function sparkline(values: number[], width = 300, height = 90): SVGSVGElement {
  const svg = svgEl("svg", {
    viewBox: "0 0 " + width + " " + height,
    class: "chart chart-line",
    preserveAspectRatio: "none",
    "aria-hidden": "true",
  }) as SVGSVGElement;
  const defs = svgEl("defs", {});
  const gradient = svgEl("linearGradient", {
    id: "spark-fill",
    x1: "0",
    y1: "0",
    x2: "0",
    y2: "1",
  });
  gradient.append(
    svgEl("stop", { offset: "0", class: "chart-stop-top" }),
    svgEl("stop", { offset: "1", class: "chart-stop-bottom" }),
  );
  defs.append(gradient);
  svg.append(defs);

  if (values.length < 2) {
    svg.append(
      svgEl("line", {
        x1: "0",
        y1: String(height - 1),
        x2: String(width),
        y2: String(height - 1),
        class: "chart-base",
      }),
    );
    return svg;
  }
  const min = Math.min(...values) - 2;
  const max = Math.max(...values) + 2;
  const span = Math.max(1, max - min);
  const step = width / (values.length - 1);
  const points: string[] = [];
  values.forEach(collectPoint);

  /** Convierte cada valor en un punto "x,y" del dibujo. */
  function collectPoint(value: number, index: number): void {
    const x = index * step;
    const y = height - 6 - ((value - min) / span) * (height - 12);
    points.push(x.toFixed(1) + "," + y.toFixed(1));
  }

  svg.append(
    svgEl("polygon", {
      points: "0," + height + " " + points.join(" ") + " " + width + "," + height,
      class: "chart-area",
    }),
    svgEl("polyline", { points: points.join(" "), class: "chart-stroke" }),
  );
  const last = points[points.length - 1].split(",");
  svg.append(svgEl("circle", { cx: last[0], cy: last[1], r: "3.5", class: "chart-dot" }));
  return svg;
}

/** Dibuja barras verticales; la última se resalta. */
export function bars(values: number[], count = 20, width = 300, height = 90): SVGSVGElement {
  const svg = svgEl("svg", {
    viewBox: "0 0 " + width + " " + height,
    class: "chart chart-bars",
    preserveAspectRatio: "none",
    "aria-hidden": "true",
  }) as SVGSVGElement;
  const recent = values.slice(-count);
  const max = Math.max(1, ...recent);
  const slot = width / count;
  const barWidth = slot * 0.62;
  for (let i = 0; i < count; i++) {
    const value = recent[i - (count - recent.length)] ?? 0;
    const h = Math.max(3, (value / max) * (height - 6));
    const isLast = i === count - 1 && recent.length > 0;
    svg.append(
      svgEl("rect", {
        x: (i * slot + (slot - barWidth) / 2).toFixed(1),
        y: (height - h).toFixed(1),
        width: barWidth.toFixed(1),
        height: h.toFixed(1),
        rx: "2",
        class: isLast ? "chart-bar chart-bar-current" : "chart-bar",
      }),
    );
  }
  return svg;
}
