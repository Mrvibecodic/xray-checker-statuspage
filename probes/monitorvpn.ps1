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
  monitorvpn delete        полностью удалить (task + ~/.xrs-probe + monitorvpn.cmd)
'@
}

# Канонизация алиасов.
$canon = $Cmd.ToLower()
switch ($canon) {
    'log'       { $canon = 'logs'; break }
    'tail'      { $canon = 'logs'; break }
    'reload'    { $canon = 'refresh'; break }
    'uninstall' { $canon = 'delete'; break }
    'rm'        { $canon = 'delete'; break }
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
