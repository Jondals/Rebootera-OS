# scripts/test-desktop-windows.ps1 - Prueba en Windows de Crear USB/actualizar (0.5): añadir una ISO y drivers a un "USB de Rebootera" FALSO (una carpeta temporal,
# ningún disco real) con la API de Rebootera Desktop. No toca ninguna unidad del usuario.
$ErrorActionPreference = "Stop"
Get-Process -Name "Rebootera Desktop" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep 2
$tmp = Join-Path $env:TEMP "rbt-fake-usb"
Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force "$tmp\rebootera" | Out-Null
Set-Content "$tmp\rebootera\rebootera-os.iso" "iso del sistema" -Encoding ascii
$src = Join-Path $env:TEMP "rbt-fake-src"
Remove-Item $src -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force "$src\drv\x64" | Out-Null
$bytes = New-Object byte[] (40MB)
(New-Object Random 5).NextBytes($bytes)
[IO.File]::WriteAllBytes("$src\W11-LTSC.iso", $bytes)
$inf = "[Version]`r`nSignature=`"`$Windows NT`$`"`r`nClass=Net`r`nProvider=Prueba`r`nDriverVer = 01/14/2025,1.0.0.1`r`n[Manufacturer]`r`nPrueba = Models,NTamd64`r`n[Models.NTamd64]`r`nTarjeta = inst, PCI\VEN_8086&DEV_15F3`r`n"
Set-Content "$src\drv\x64\net.inf" $inf -Encoding ascii
Set-Content "$src\drv\x64\net.sys" "sys" -Encoding ascii
$urlFile = Join-Path $env:TEMP "rbt-desktop-url.txt"
Remove-Item $urlFile -ErrorAction SilentlyContinue
$p = Start-Process (Join-Path (Split-Path $PSScriptRoot) "out\Rebootera Desktop.exe") -ArgumentList "-url-file", $urlFile, "-usb-root", $tmp -PassThru
for ($i = 0; $i -lt 30 -and -not (Test-Path $urlFile); $i++) { Start-Sleep -Milliseconds 500 }
$url = (Get-Content $urlFile -Raw).Trim()
$base = $url -replace "/auth\?token=.*", ""
$null = Invoke-WebRequest -UseBasicParsing -Uri $url -SessionVariable s
$h = @{ "X-Rebootera" = "1" }
function Api($method, $path, $body) {
  try {
    if ($body) { return Invoke-RestMethod -Method $method -Uri "$base$path" -WebSession $s -Headers $h -ContentType "application/json" -Body ($body | ConvertTo-Json -Depth 5 -Compress) }
    return Invoke-RestMethod -Method $method -Uri "$base$path" -WebSession $s -Headers $h
  } catch { return @{ error = $_.ErrorDetails.Message } }
}
"modo: $((Api GET '/api/mode') | ConvertTo-Json -Compress)"
$r = Api POST "/api/desktop/apply" @{ root = "$tmp\"; system = "keep"; images = @(@{ path = "$src\W11-LTSC.iso"; category = "windows" }); driverDirs = @("$src\drv") }
"apply: $($r | ConvertTo-Json -Compress)"
for ($i = 0; $i -lt 60; $i++) { Start-Sleep -Milliseconds 400; $st = Api GET "/api/desktop/create"; if (-not $st.running) { break } }
"estado: $($st | ConvertTo-Json -Compress)"
$same = (Get-FileHash "$src\W11-LTSC.iso").Hash -eq (Get-FileHash "$tmp\isos\windows\W11-LTSC.iso").Hash
"ISO copiada identica: $same"
"drivers copiados: $((Test-Path "$tmp\drivers\drv\x64\net.inf") -and (Test-Path "$tmp\drivers\drv\x64\net.sys"))"
"partes sueltas: $((Get-ChildItem $tmp -Recurse -Filter *.part | Measure-Object).Count)"
$sys = Get-Content "$tmp\rebootera\rebootera-os.iso" -Raw
"sistema intacto: $($sys.Trim() -eq 'iso del sistema')"
$d = Api GET "/api/desktop/drivers"
"drivers listados: $($d.packages.Count) en $($d.root)"
$none = Api POST "/api/desktop/apply" @{ root = "$tmp\"; system = "keep" }
"sin nada que hacer: $($none | ConvertTo-Json -Compress)"
$rm = Api POST "/api/desktop/drivers/remove" @{ name = "drv" }
"tras quitar: $($rm.packages.Count) paquetes; carpeta existe: $(Test-Path "$tmp\drivers\drv")"
$evil = Api POST "/api/desktop/drivers/remove" @{ name = "..\rebootera" }
"quitar ruta con ..: $($evil | ConvertTo-Json -Compress); sistema sigue: $(Test-Path "$tmp\rebootera\rebootera-os.iso")"
Stop-Process -Id $p.Id -Force
Start-Sleep 1
Remove-Item $tmp, $src -Recurse -Force
"limpio"
