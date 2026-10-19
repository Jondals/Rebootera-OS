# * Prueba de extremo a extremo de "instalador en carpetas a ISO" con el Rebootera Desktop.exe real en Windows,
# * SIN ventana (-no-window) y contra un USB FALSO (una carpeta temporal): no toca ningún disco.
# * Crea una carpeta con el aspecto de un Windows extraído por Rufus, pide a la API que haga la ISO
# * (IMAPI de Windows) y comprueba la ISO: firma ISO 9660, catálogo El Torito con BIOS y EFI, y la biblioteca.
# *
# *   scripts\test-installer-windows.ps1 [-Exe "ruta\Rebootera Desktop.exe"]
param([string]$Exe = (Join-Path $PSScriptRoot "..\..\out\Rebootera Desktop.exe"))
$ErrorActionPreference = "Stop"
$Exe = (Resolve-Path $Exe).Path
$work = Join-Path $env:TEMP ("rbt-installer-" + [guid]::NewGuid().ToString("N").Substring(0, 8))
$fake = Join-Path $work "usb"
$folder = Join-Path $work "Win11 Pro"
foreach ($d in @("$fake\rebootera\state", "$fake\isos\windows", "$fake\isos\linux", "$fake\drivers")) { New-Item -ItemType Directory -Force $d | Out-Null }
Set-Content "$fake\rebootera\rebootera-os.iso" "iso" -Encoding ascii
$files = @{ "bootmgr" = 2048; "boot\etfsboot.com" = 4096; "efi\microsoft\boot\efisys.bin" = 4096; "sources\boot.wim" = 65536; "sources\install.wim" = 131072; "setup.exe" = 1024 }
foreach ($name in $files.Keys) {
  $path = Join-Path $folder $name
  New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
  [IO.File]::WriteAllBytes($path, (New-Object byte[] $files[$name]))
}
$urlFile = Join-Path $work "url.txt"
$p = Start-Process $Exe -ArgumentList "-no-window", "-url-file", $urlFile, "-usb-root", $fake -PassThru
$failures = 0
# * Check anota una comprobación como correcta o fallida y la enseña.
function Check([string]$what, [bool]$ok) {
  if ($ok) { "OK    $what" } else { "FALLO $what"; $script:failures++ }
}
try {
  for ($i = 0; $i -lt 100 -and -not (Test-Path $urlFile); $i++) { Start-Sleep -Milliseconds 100 }
  $url = (Get-Content $urlFile -Raw).Trim()
  $base = $url -replace "/auth\?token=.*", ""
  $null = Invoke-WebRequest -UseBasicParsing -Uri $url -SessionVariable s
  $headers = @{ "X-Rebootera" = "1"; "Content-Type" = "application/json" }
  $body = (@{ device = ""; path = $folder } | ConvertTo-Json -Compress)
  $info = Invoke-RestMethod -Uri "$base/api/installer/inspect" -Method Post -Body $body -Headers $headers -WebSession $s
  Check "detecta el instalador de Windows" ($info.kind -eq "windows" -and $info.biosBoot -ne "" -and $info.uefiBoot -ne "")
  $job = Invoke-RestMethod -Uri "$base/api/transfers/iso" -Method Post -Body $body -Headers $headers -WebSession $s
  Check "la ISO entra en la cola" ($job.kind -eq "iso")
  $status = ""
  for ($i = 0; $i -lt 120; $i++) {
    Start-Sleep -Milliseconds 500
    $jobs = Invoke-RestMethod -Uri "$base/api/transfers" -WebSession $s
    $status = ($jobs | Where-Object { $_.id -eq $job.id }).status
    if ($status -in @("done", "failed", "cancelled")) { break }
  }
  Check "el trabajo termina bien (estado: $status)" ($status -eq "done")
  $iso = Join-Path $fake "isos\windows\Win11 Pro.iso"
  if (Test-Path $iso) {
    $bytes = [IO.File]::ReadAllBytes($iso)
    Check "firma ISO 9660 (CD001)" ([Text.Encoding]::ASCII.GetString($bytes, 0x8001, 5) -eq "CD001")
    $torito = [Text.Encoding]::ASCII.GetString($bytes, 0x8800 + 7, 23)
    Check "catálogo de arranque El Torito" ($torito -eq "EL TORITO SPECIFICATION")
    $catalog = [BitConverter]::ToUInt32($bytes, 0x8800 + 71) * 2048
    Check "plataformas BIOS (0x00) y UEFI (0xEF)" ($bytes[$catalog + 1] -eq 0 -and $bytes[$catalog + 65] -eq 0xEF)
  } else {
    Check "la ISO existe en isos\windows" $false
  }
  $library = Invoke-RestMethod -Uri "$base/api/library" -WebSession $s
  Check "aparece en la biblioteca" (@($library | Where-Object { $_.name -eq "Win11 Pro.iso" }).Count -eq 1)
} finally {
  Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  Start-Sleep 1
  Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
}
"Fallos: $failures"
exit $failures
