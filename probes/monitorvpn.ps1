<#
.SYNOPSIS
  CLI для управления probe-агентом xray-checker-statuspage на Windows.

.DESCRIPTION
  Команды: start | stop | restart | status | logs | refresh | delete.
  Агент работает через Scheduled Task `XrayCheckerProbe`. Конфиг
  (URL и PROBE_TOKEN) лежит в %USERPROFILE%\.xrs-probe\config.env.

.EXAMPLE
  PS> monitorvpn status
  PS> monitorvpn refresh
  PS> monitorvpn logs
#>
param([string]$Cmd = 'status')

$ErrorActionPreference = 'SilentlyContinue'

$TaskName   = 'XrayCheckerProbe'
$AppDir     = Join-Path $env:USERPROFILE '.xrs-probe'
$LogFile    = Join-Path $AppDir 'agent.log'
$ConfigFile = Join-Path $AppDir 'config.env'
$LocalBin   = Join-Path $env:USERPROFILE '.local\bin'
$AgentRaw   = 'https://raw.githubusercontent.com/Mrvibecodic/xray-checker-statuspage/main/probes/agent.py'
$MonitorRaw = 'https://raw.githubusercontent.com/Mrvibecodic/xray-checker-statuspage/main/probes/monitorvpn.ps1'

function Write-G ([string]$msg) { Write-Host $msg -ForegroundColor Green }
function Write-Y ([string]$msg) { Write-Host $msg -ForegroundColor Yellow }
function Write-R ([string]$msg) { Write-Host $msg -ForegroundColor Red }

function Get-Config {
    if (-not (Test-Path $ConfigFile)) { return $null }
    $h = @{}
    Get-Content $ConfigFile | ForEach-Object {
        if ($_ -match '^([^=]+)=(.*)$') {
            $h[$Matches[1].Trim()] = $Matches[2].Trim()
        }
    }
    return $h
}
function Get-Task { Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue }
function Usage {
@'
monitorvpn — управление probe-агентом xray-checker-statuspage

Команды:
  monitorvpn start         запустить агент (Scheduled Task)
  monitorvpn stop          остановить
  monitorvpn restart       перезапустить
  monitorvpn status        статус + последние строки лога
  monitorvpn logs          tail -f лога
  monitorvpn refresh       попросить сервер перечитать подписку
  monitorvpn update        обновить agent.py и сам monitorvpn (без переустановки)
  monitorvpn xray-update   скачать свежий xray-core последнего релиза
  monitorvpn help          показать эту справку
  monitorvpn delete        полностью удалить (task + ~/.xrs-probe + monitorvpn.cmd)
'@
}

# Канонизация алиасов.
$canon = $Cmd.ToLower()
switch ($canon) {
    'log'       { $canon = 'logs'; break }
    'tail'      { $canon = 'logs'; break }
    'reload'    { $canon = 'refresh'; break }
    'uninstall'   { $canon = 'delete'; break }
    'rm'          { $canon = 'delete'; break }
    'update-xray' { $canon = 'xray-update'; break }
    'self-update' { $canon = 'update'; break }
    'help'      { $canon = ''; break }
    '-h'        { $canon = ''; break }
    '--help'    { $canon = ''; break }
}

switch ($canon) {
    'start' {
        $task = Get-Task
        if (-not $task) { Write-R "✗ Scheduled Task '$TaskName' не найден. Запусти install-windows.ps1."; exit 1 }
        Start-ScheduledTask -TaskName $TaskName
        Write-G '✓ агент запущен'
    }
    'stop' {
        $task = Get-Task
        if (-not $task) { Write-Y 'агент не установлен'; exit 0 }
        Stop-ScheduledTask -TaskName $TaskName
        Write-G '✓ агент остановлен'
    }
    'restart' {
        $task = Get-Task
        if (-not $task) { Write-R "✗ Scheduled Task '$TaskName' не найден."; exit 1 }
        Stop-ScheduledTask -TaskName $TaskName
        Start-Sleep -Seconds 1
        Start-ScheduledTask -TaskName $TaskName
        Write-G '✓ перезапущено'
    }
    'status' {
        $task = Get-Task
        if (-not $task) { Write-R "✗ агент не установлен (нет Scheduled Task '$TaskName')"; exit 0 }
        $info = Get-ScheduledTaskInfo -TaskName $TaskName
        Write-G "✓ Scheduled Task: $TaskName"
        Write-Host "  State:       $($task.State)"
        if ($info.LastRunTime) {
            Write-Host "  Last run:    $($info.LastRunTime)"
            Write-Host "  Last result: 0x$('{0:X}' -f $info.LastTaskResult) ($($info.LastTaskResult))"
        }
        if (Test-Path $LogFile) {
            Write-Host ''
            Write-Y "Последние строки лога ($LogFile):"
            Get-Content -Path $LogFile -Tail 5
        } else {
            Write-Y "лог-файл ещё не создан: $LogFile"
        }
    }
    'logs' {
        if (-not (Test-Path $LogFile)) { Write-R "лог-файл не найден: $LogFile"; exit 1 }
        Get-Content -Path $LogFile -Tail 30 -Wait
    }
    'refresh' {
        $cfg = Get-Config
        if (-not $cfg -or -not $cfg.STATUSPAGE_URL -or -not $cfg.PROBE_TOKEN) {
            Write-R "не могу прочитать STATUSPAGE_URL/PROBE_TOKEN из $ConfigFile"
            exit 1
        }
        Write-Y 'запрашиваю force-refresh подписки на сервере…'
        $url = $cfg.STATUSPAGE_URL.TrimEnd('/') + '/api/probe/targets?force=1'
        try {
            $resp = Invoke-RestMethod -Method GET -Uri $url `
                -Headers @{ 'X-Probe-Token' = $cfg.PROBE_TOKEN } -TimeoutSec 20
            $n = ($resp.targets | Measure-Object).Count
            $when = ([DateTimeOffset]::FromUnixTimeSeconds([int64]$resp.fetchedAt)).LocalDateTime
            Write-G "✓ сервер вернул $n таргетов (fetchedAt=$when)"
            Write-Host '  пробник подхватит обновлённый список со следующим циклом.'
        } catch {
            Write-R "не удалось: $($_.Exception.Message)"
            exit 1
        }
    }
    'update' {
        # Обновляет agent.py и сам monitorvpn.ps1 до свежей версии из main.
        # Регистрация Scheduled Task и токен (config.env) не трогаются.
        # После обновления — soft restart Scheduled Task, если она запущена.
        Write-Y 'качаю свежий agent.py…'
        $agentTmp = "$AppDir\agent.py.new"
        try {
            Invoke-WebRequest -Uri $AgentRaw -OutFile $agentTmp -UseBasicParsing
        } catch {
            Write-R "не удалось скачать agent.py: $($_.Exception.Message)"; exit 1
        }
        $head = (Get-Content -Path $agentTmp -TotalCount 1)
        if ($head -notmatch '^#!.*python') {
            Write-R 'скачанный agent.py не похож на python-скрипт (404? proxy-перехват?)'
            Remove-Item -Force $agentTmp
            exit 1
        }
        Move-Item -Force $agentTmp "$AppDir\agent.py"
        Write-G '  ✓ agent.py обновлён'
        Write-Y 'качаю свежий monitorvpn.ps1…'
        $monTmp = "$AppDir\monitorvpn.ps1.new"
        try {
            Invoke-WebRequest -Uri $MonitorRaw -OutFile $monTmp -UseBasicParsing
            $monHead = (Get-Content -Path $monTmp -TotalCount 5) -join "`n"
            if ($monHead -match 'monitorvpn') {
                Move-Item -Force $monTmp "$AppDir\monitorvpn.ps1"
                Write-G '  ✓ monitorvpn.ps1 обновлён'
            } else {
                Write-Y '  ! скачанный monitorvpn.ps1 невалиден, пропускаю'
                Remove-Item -Force $monTmp -ErrorAction SilentlyContinue
            }
        } catch {
            Write-Y "  ! не удалось скачать monitorvpn.ps1 (не критично): $($_.Exception.Message)"
        }
        $task = Get-Task
        if ($task -and $task.State -eq 'Running') {
            Write-Y 'перезапускаю агент, чтобы новая версия начала работать…'
            Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
            Start-Sleep -Seconds 1
            Start-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        }
        Write-G '✓ обновление завершено.'
    }
    'help' {
        Usage
    }
    'xray-update' {
        $arch = if ([Environment]::Is64BitOperatingSystem) {
            if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') {
                'Xray-windows-arm64-v8a.zip'
            } else { 'Xray-windows-64.zip' }
        } else { 'Xray-windows-32.zip' }
        $xrayVer = if ($env:XRAY_VERSION) { $env:XRAY_VERSION } else { 'v25.12.8' }
        $url = if ($xrayVer -eq 'latest') {
            "https://github.com/XTLS/Xray-core/releases/latest/download/$arch"
        } else {
            "https://github.com/XTLS/Xray-core/releases/download/$xrayVer/$arch"
        }
        $tmpZip = Join-Path $env:TEMP "xray-$([System.IO.Path]::GetRandomFileName()).zip"
        $tmpDir = Join-Path $env:TEMP "xray-$([System.IO.Path]::GetRandomFileName())"
        $xrayPath = Join-Path $AppDir 'xray.exe'
        Write-Y "качаю $url…"
        try {
            Invoke-WebRequest -Uri $url -OutFile $tmpZip -UseBasicParsing
            Expand-Archive -Path $tmpZip -DestinationPath $tmpDir -Force
            $exe = Get-ChildItem -Path $tmpDir -Recurse -Filter 'xray.exe' | Select-Object -First 1
            if (-not $exe) { Write-R "не нашёл xray.exe в архиве"; exit 1 }
            Copy-Item -Force $exe.FullName $xrayPath
            Write-G "✓ xray обновлён ($xrayPath)"
            $task = Get-Task
            if ($task -and $task.State -eq 'Running') {
                Write-Y 'перезапускаю агент чтобы новый xray начал использоваться…'
                Stop-ScheduledTask -TaskName $TaskName
                Start-Sleep -Seconds 1
                Start-ScheduledTask -TaskName $TaskName
            }
        } catch {
            Write-R "не удалось: $($_.Exception.Message)"; exit 1
        } finally {
            Remove-Item -Force -Recurse -ErrorAction SilentlyContinue $tmpZip, $tmpDir
        }
    }
    'delete' {
        Write-Y 'Удаление…'
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
        if (Test-Path $AppDir) { Remove-Item -Recurse -Force $AppDir -ErrorAction SilentlyContinue }
        $cmdWrapper = Join-Path $LocalBin 'monitorvpn.cmd'
        if (Test-Path $cmdWrapper) { Remove-Item -Force $cmdWrapper -ErrorAction SilentlyContinue }
        Write-G "✓ удалено. Лог-файл (если был) остался в $LogFile."
    }
    '' {
        Usage
    }
    default {
        Write-R "неизвестная команда: $Cmd"
        Write-Host ''
        Usage
        exit 2
    }
}
