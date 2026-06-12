<#
.SYNOPSIS
  Установщик probe-агента xray-checker-statuspage для Windows.

.DESCRIPTION
  Регистрирует пробник на сервере (POST /api/admin/probes mode=merge — история
  сохраняется при повторных запусках), кладёт agent.py в %USERPROFILE%\.xrs-probe,
  создаёт Scheduled Task "XrayCheckerProbe" с триггером "At log on" и автоматическим
  перезапуском при падении. Добавляет команду `monitorvpn` в %USERPROFILE%\.local\bin
  и регистрирует её в User PATH.

.PARAMETER StatuspageUrl
  URL статус-страницы (https://...). Если не задан — спросит.

.PARAMETER AdminToken
  ADMIN_TOKEN статус-страницы (одноразово нужен для регистрации). Если не задан — спросит.

.PARAMETER ProbeName
  Имя пробника. По умолчанию — "Win-<COMPUTERNAME>".

.PARAMETER Interval
  Период цикла в секундах. По умолчанию 60.

.PARAMETER ExpectCountry
  Ожидаемый ISO-код страны (для VPN-стража). По умолчанию RU.

.PARAMETER Uninstall
  Полностью снести агент.

.EXAMPLE
  PS> .\install-windows.ps1
  PS> .\install-windows.ps1 -StatuspageUrl https://status.example.com -AdminToken xxx
  PS> .\install-windows.ps1 -Uninstall
#>
[CmdletBinding()]
param(
    [string]$StatuspageUrl = "",
    [string]$AdminToken = "",
    [string]$ProbeName = "",
    [int]$Interval = 60,
    [string]$ExpectCountry = "RU",
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'

$TaskName  = 'XrayCheckerProbe'
$AppDir    = Join-Path $env:USERPROFILE '.xrs-probe'
$LogFile   = Join-Path $AppDir 'agent.log'
$LocalBin  = Join-Path $env:USERPROFILE '.local\bin'
$AgentRaw  = 'https://raw.githubusercontent.com/Mrvibecodic/xray-checker-statuspage/main/probes/agent.py'
$MonitorRaw = 'https://raw.githubusercontent.com/Mrvibecodic/xray-checker-statuspage/main/probes/monitorvpn.ps1'

function Write-G ([string]$msg) { Write-Host $msg -ForegroundColor Green }
function Write-Y ([string]$msg) { Write-Host $msg -ForegroundColor Yellow }
function Write-R ([string]$msg) { Write-Host $msg -ForegroundColor Red }

function Get-PythonExe {
    # Нужен РЕАЛЬНЫЙ python.exe, а НЕ launcher py.exe. Причина: py.exe читает
    # shebang '#!/usr/bin/env python3' в agent.py и уходит запускать `python3`,
    # который на многих системах — заглушка Microsoft Store («Python was not
    # found»). python.exe shebang не читает и запускает скрипт как есть.
    # Поэтому: находим интерпретатор, спрашиваем у него sys.executable и
    # возвращаем именно его (отсеивая WindowsApps-заглушки).
    foreach ($cand in @('py', 'python', 'python3')) {
        $c = Get-Command $cand -ErrorAction SilentlyContinue
        if (-not $c) { continue }
        if ($c.Path -like '*\WindowsApps\*') { continue }   # стор-заглушка
        try {
            $real = (& $c.Path -c "import sys; print(sys.executable)" 2>$null | Select-Object -First 1)
        } catch { $real = $null }
        if ($real -and (Test-Path $real) -and ($real -notlike '*\WindowsApps\*')) {
            return $real.Trim()
        }
    }
    return $null
}

if ($Uninstall) {
    Write-Y 'Удаление…'
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
    if (Test-Path $AppDir) { Remove-Item -Recurse -Force $AppDir -ErrorAction SilentlyContinue }
    $monitorCmd = Join-Path $LocalBin 'monitorvpn.cmd'
    if (Test-Path $monitorCmd) { Remove-Item -Force $monitorCmd -ErrorAction SilentlyContinue }
    Write-G "✓ удалено. Лог-файл (если был) остался в $LogFile."
    exit 0
}

$python = Get-PythonExe
if (-not $python) {
    Write-R "Python не найден. Установи Python 3.10+ — самый простой путь:"
    Write-Host "    winget install Python.Python.3.12"
    Write-Host "  потом перезайди в PowerShell и запусти установщик снова."
    exit 1
}
Write-G "✓ Python: $python"

if (-not $StatuspageUrl) {
    $StatuspageUrl = Read-Host 'URL статус-страницы (https://...)'
}
if (-not $AdminToken) {
    $sec = Read-Host 'ADMIN_TOKEN статус-страницы' -AsSecureString
    $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec)
    try { $AdminToken = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($bstr) }
    finally { [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
}
if (-not $ProbeName) {
    $default = "Win-$env:COMPUTERNAME"
    $tmp = Read-Host "Имя пробника [$default]"
    if ($tmp) { $ProbeName = $tmp } else { $ProbeName = $default }
}

if (-not $StatuspageUrl -or -not $AdminToken) {
    Write-R 'URL и ADMIN_TOKEN обязательны.'
    exit 1
}

$StatuspageUrl = $StatuspageUrl.TrimEnd('/')

Write-G '→ регистрируем пробник на сервере (mode=merge — история сохраняется)…'
$body = @{ name = $ProbeName; mode = 'merge' } | ConvertTo-Json -Compress
$headers = @{ 'X-Admin-Token' = $AdminToken; 'Content-Type' = 'application/json; charset=utf-8' }
try {
    $resp = Invoke-RestMethod -Method POST -Uri "$StatuspageUrl/api/admin/probes" `
                              -Headers $headers -Body $body -TimeoutSec 20
} catch {
    Write-R "Регистрация не удалась: $($_.Exception.Message)"
    Write-R 'Проверь URL и ADMIN_TOKEN.'
    exit 1
}
$ProbeId    = $resp.probe_id
$ProbeToken = $resp.probe_token
$reused     = if ($resp.reused) { ' (переиспользован)' } else { '' }
Write-Host "  probe_id: $ProbeId$reused"

Write-G "→ устанавливаем агент в $AppDir…"
New-Item -ItemType Directory -Force -Path $AppDir | Out-Null
$selfDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$agentSrc = Join-Path $selfDir 'agent.py'
if (Test-Path $agentSrc) {
    Copy-Item -Force $agentSrc (Join-Path $AppDir 'agent.py')
    Write-Host "  agent.py скопирован из $selfDir"
} else {
    Invoke-WebRequest -Uri $AgentRaw -OutFile (Join-Path $AppDir 'agent.py') -UseBasicParsing
    Write-Host '  agent.py скачан с GitHub'
}

$monitorPs1 = Join-Path $AppDir 'monitorvpn.ps1'
$monitorSrc = Join-Path $selfDir 'monitorvpn.ps1'
if (Test-Path $monitorSrc) {
    Copy-Item -Force $monitorSrc $monitorPs1
} else {
    try {
        Invoke-WebRequest -Uri $MonitorRaw -OutFile $monitorPs1 -UseBasicParsing
    } catch {
        Write-Y 'не удалось скачать monitorvpn.ps1 (необязательно)'
    }
}

# xray-core: качаем последний релиз github.com/XTLS/Xray-core под архитектуру.
# Без него агент не сможет валидировать конфиги — только пустые отчёты с ошибкой.
$XrayPath = Join-Path $AppDir 'xray.exe'
if (-not (Test-Path $XrayPath)) {
    Write-G '→ ставим xray-core…'
    $arch = if ([Environment]::Is64BitOperatingSystem) {
        if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') {
            'Xray-windows-arm64-v8a.zip'
        } else { 'Xray-windows-64.zip' }
    } else { 'Xray-windows-32.zip' }
    # Пинимся на v25.12.8 — последняя до 26.x с обязательным password в REALITY.
    # Override: $env:XRAY_VERSION="latest" перед запуском.
    $xrayVer = if ($env:XRAY_VERSION) { $env:XRAY_VERSION } else { 'v25.12.8' }
    $xrayUrl = if ($xrayVer -eq 'latest') {
        "https://github.com/XTLS/Xray-core/releases/latest/download/$arch"
    } else {
        "https://github.com/XTLS/Xray-core/releases/download/$xrayVer/$arch"
    }
    $tmpZip = Join-Path $env:TEMP "xray-$([System.IO.Path]::GetRandomFileName()).zip"
    $tmpDir = Join-Path $env:TEMP "xray-$([System.IO.Path]::GetRandomFileName())"
    try {
        Invoke-WebRequest -Uri $xrayUrl -OutFile $tmpZip -UseBasicParsing
        Expand-Archive -Path $tmpZip -DestinationPath $tmpDir -Force
        $exe = Get-ChildItem -Path $tmpDir -Recurse -Filter 'xray.exe' | Select-Object -First 1
        if ($exe) {
            Copy-Item -Force $exe.FullName $XrayPath
            Write-Host "  ✓ xray.exe установлен"
        } else {
            Write-Y "не нашёл xray.exe в архиве $arch. Поставь вручную в $XrayPath"
        }
    } catch {
        Write-Y "не удалось скачать xray-core ($($_.Exception.Message)). Поставь вручную в $XrayPath"
    } finally {
        Remove-Item -Force -Recurse -ErrorAction SilentlyContinue $tmpZip, $tmpDir
    }
}

# Конфиг с env-переменными. Хранится с правами текущего пользователя.
$configContent = @"
STATUSPAGE_URL=$StatuspageUrl
PROBE_TOKEN=$ProbeToken
INTERVAL=$Interval
EXPECT_COUNTRY=$ExpectCountry
"@
$configContent | Out-File -FilePath (Join-Path $AppDir 'config.env') -Encoding UTF8 -NoNewline

# Wrapper читает config.env, ставит env-переменные и запускает agent.py.
# Логи перенаправляются в agent.log (append).
$wrapperContent = @"
`$ErrorActionPreference = 'SilentlyContinue'
Get-Content '$AppDir\config.env' | ForEach-Object {
    if (`$_ -match '^([^=]+)=(.*)$') {
        Set-Item -Path "Env:`$(`$Matches[1].Trim())" -Value `$Matches[2].Trim()
    }
}
& '$python' '$AppDir\agent.py' *>> '$LogFile'
"@
$wrapperContent | Out-File -FilePath (Join-Path $AppDir 'run-agent.ps1') -Encoding UTF8

Write-G "→ регистрируем Scheduled Task '$TaskName' (триггер: при входе в систему)…"
# Удалим существующую если была
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

$psExe = (Get-Command powershell.exe).Source
# -Argument ждёт ОДНУ строку со всеми аргументами (не массив — иначе
# ParameterArgumentTransformationError). Путь к скрипту в кавычках на случай
# пробелов в имени пользователя.
$taskArgs = '-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + "$AppDir\run-agent.ps1" + '"'
$action = New-ScheduledTaskAction -Execute $psExe -Argument $taskArgs
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) `
    -StartWhenAvailable
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Settings $settings -Principal $principal | Out-Null

# Запускаем сейчас
Start-ScheduledTask -TaskName $TaskName

# Команда monitorvpn в PATH. Через CMD-обёртку, потому что
# она работает из любого shell'а (CMD/PowerShell/Git Bash).
Write-G '→ устанавливаем команду monitorvpn…'
New-Item -ItemType Directory -Force -Path $LocalBin | Out-Null
$cmdWrapper = @"
@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%USERPROFILE%\.xrs-probe\monitorvpn.ps1" %*
"@
$cmdWrapper | Out-File -FilePath (Join-Path $LocalBin 'monitorvpn.cmd') -Encoding ASCII

# Добавляем %USERPROFILE%\.local\bin в User PATH, если ещё нет.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($null -eq $userPath) { $userPath = '' }
if ($userPath -notlike "*$LocalBin*") {
    $newPath = if ($userPath) { "$userPath;$LocalBin" } else { $LocalBin }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    Write-Host "  $LocalBin добавлен в User PATH (перезайди в PowerShell, чтобы применилось)"
}

Write-Host ''
Write-G '✓ агент запущен. При следующих логинах будет стартовать сам.'
Write-Host "   Управление:  monitorvpn {start|stop|restart|status|logs|refresh|update|xray-update|delete}"
Write-Host "   Логи:        $LogFile"
Write-Host "   Снести:      monitorvpn delete   (или: .\install-windows.ps1 -Uninstall)"
Write-Host ''
Write-Y 'Подсказка: первая строка лога должна быть «start: interval=...» через ~5 сек.'
Write-Y '            Если получаешь "monitorvpn: command not found" — перезайди в shell.'
