# * site.ps1: ver la página web (site/, Astro + Tailwind) en el navegador. Compila la página (pnpm build), copia las
# * capturas de docs/images y arranca un servidor local mínimo en http://localhost:PUERTO/ (por defecto el 8080)
# * que sirve hasta Ctrl+C. Necesita Node y pnpm.
# *
# *   powershell -ExecutionPolicy Bypass -File scripts\dev\site.ps1 [-Port 8080] [-NoOpen] [-NoBuild]
param([int]$Port = 8080, [switch]$NoOpen, [switch]$NoBuild)
$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$site = Join-Path $repo "site"
$root = Join-Path $site "dist"

# * Las capturas viven en docs/images; la página las copia a public/images antes de compilar.
New-Item -ItemType Directory -Force (Join-Path $site "public\images") | Out-Null
Copy-Item (Join-Path $repo "docs\images\*.png") (Join-Path $site "public\images") -Force

if (-not $NoBuild) {
  Push-Location $site
  try {
    if (-not (Test-Path "node_modules")) { pnpm install }
    pnpm build
    if ($LASTEXITCODE -ne 0) { throw "La compilación de la página falló." }
  } finally {
    Pop-Location
  }
}

$types = @{
  ".html" = "text/html; charset=utf-8"; ".css" = "text/css"; ".js" = "text/javascript"; ".svg" = "image/svg+xml"
  ".png" = "image/png"; ".woff2" = "font/woff2"; ".txt" = "text/plain; charset=utf-8"
}

# * Send escribe en la respuesta un archivo de site/dist (o un 404 si no existe o intenta salir de la carpeta).
function Send($context) {
  $relative = [Uri]::UnescapeDataString($context.Request.Url.AbsolutePath.TrimStart("/"))
  if ($relative -eq "") { $relative = "index.html" }
  $path = [IO.Path]::GetFullPath((Join-Path $root $relative))
  if (-not $path.StartsWith($root) -or -not (Test-Path $path -PathType Leaf)) {
    $context.Response.StatusCode = 404
    $context.Response.Close()
    return
  }
  $type = $types[[IO.Path]::GetExtension($path).ToLower()]
  if (-not $type) { $type = "application/octet-stream" }
  $bytes = [IO.File]::ReadAllBytes($path)
  $context.Response.ContentType = $type
  $context.Response.ContentLength64 = $bytes.Length
  $context.Response.OutputStream.Write($bytes, 0, $bytes.Length)
  $context.Response.Close()
}

$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:$Port/")
$listener.Start()
Write-Host "Página en http://localhost:$Port/  (Ctrl+C para parar)"
if (-not $NoOpen) { Start-Process "http://localhost:$Port/" }
try {
  while ($listener.IsListening) { Send ($listener.GetContext()) }
} finally {
  $listener.Stop()
}
