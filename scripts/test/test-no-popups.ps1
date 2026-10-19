# scripts/test/test-no-popups.ps1 - Comprueba que Rebootera Desktop no abre ninguna ventana de PowerShell/consola.
# Abre la sección "Crear USB" (que lista los discos con PowerShell) y vigila, cada 10 ms durante unos segundos,
# si aparece alguna ventana de powershell.exe, conhost.exe o cmd.exe. No toca ningún disco: solo los lista.
#   test-no-popups.ps1 [RUTA_AL_EXE]
param([string]$Exe = (Get-ChildItem (Join-Path (Split-Path (Split-Path $PSScriptRoot)) "out") -Filter "Rebootera-Desktop-*.exe" | Sort-Object LastWriteTime -Descending | Select-Object -First 1).FullName)
$ErrorActionPreference = "Stop"
Get-Process -Name "Rebootera Desktop" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep 2
$names = @("powershell", "pwsh", "conhost", "cmd", "WindowsTerminal", "OpenConsole")
$before = @(Get-Process | Where-Object { $names -contains $_.Name -and $_.MainWindowHandle -ne 0 } | ForEach-Object { $_.Id })
$p = Start-Process $Exe -ArgumentList "-route", "create" -PassThru
$seen = @{}
$polls = 0
$deadline = (Get-Date).AddSeconds(14)
while ((Get-Date) -lt $deadline) {
  $polls++
  foreach ($proc in (Get-Process | Where-Object { $names -contains $_.Name -and $_.MainWindowHandle -ne 0 })) {
    if ($before -notcontains $proc.Id -and -not $seen.ContainsKey($proc.Id)) { $seen[$proc.Id] = "$($proc.Name) '$($proc.MainWindowTitle)'" }
  }
  Start-Sleep -Milliseconds 10
}
Stop-Process -Id $p.Id -Force
"sondeos: $polls; ventanas de consola nuevas: $($seen.Count)"
$seen.Values | ForEach-Object { "  $_" }
if ($seen.Count -gt 0) { exit 1 }
