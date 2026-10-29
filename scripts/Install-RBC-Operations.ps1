$ErrorActionPreference = 'Stop'
$script = Join-Path $PSScriptRoot 'Invoke-RBC-Operations.ps1'
$projectRoot = Split-Path -Parent $PSScriptRoot
$logRoot = Join-Path $projectRoot '.runtime/operations'
New-Item -ItemType Directory -Force $logRoot | Out-Null
$powershell = Join-Path $env:SystemRoot 'System32/WindowsPowerShell/v1.0/powershell.exe'
$operator = [Security.Principal.WindowsIdentity]::GetCurrent().Name
$principal = New-ScheduledTaskPrincipal -UserId $operator -LogonType Interactive -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -MultipleInstances IgnoreNew -ExecutionTimeLimit (New-TimeSpan -Minutes 20) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
foreach ($action in @('Monitor','Backup','RestoreTest')) {
    $log = Join-Path $logRoot "task-$action.log"
    $command = "& '$script' -Action '$action' *> '$log'; if (-not `$?) { exit 1 }"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
    # Scope policy to this process, as in the existing project launcher;
    # do not modify the user's machine-wide PowerShell policy.
    $taskAction = New-ScheduledTaskAction -Execute $powershell -WorkingDirectory $projectRoot -Argument "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -EncodedCommand $encoded"
    $trigger = switch ($action) {
        Monitor { @( (New-ScheduledTaskTrigger -AtLogOn -User $operator), (New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(1) -RepetitionInterval (New-TimeSpan -Minutes 5))) }
        Backup { New-ScheduledTaskTrigger -Daily -At '02:00' }
        RestoreTest { New-ScheduledTaskTrigger -Weekly -DaysOfWeek Sunday -At '03:00' }
    }
    Register-ScheduledTask -TaskName "RBC-$action" -Action $taskAction -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null
}
Write-Host 'Installed RBC monitoring, daily backup and weekly restore testing (runs while operator is signed in).'
