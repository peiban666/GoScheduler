$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$runtime = Join-Path $root '.runtime'
$configFile = Join-Path $runtime 'runtime.json'
if (!(Test-Path -LiteralPath $configFile)) {
    throw '当前电脑尚未初始化本地数据库；部署到其他电脑时请按 README 安装 Go 服务及数据库。'
}
$config = Get-Content -LiteralPath $configFile -Raw | ConvertFrom-Json
$database = [IO.Path]::GetFullPath($config.databaseExecutable)
$tools = [IO.Path]::GetFullPath((Join-Path (Split-Path $root -Parent) '.tools')) + [IO.Path]::DirectorySeparatorChar
if (!$database.StartsWith($tools, [StringComparison]::OrdinalIgnoreCase) -or
    [IO.Path]::GetFileName($database) -ne 'mariadbd.exe' -or !(Test-Path -LiteralPath $database)) {
    throw '数据库程序路径不符合当前工作区配置。'
}
$server = Join-Path $runtime 'bin\goscheduler.exe'

function Test-ExpectedListener([int]$port, [string]$executable) {
    $listener = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    if (!$listener) { return $false }
    foreach ($connection in $listener) {
        $process = Get-CimInstance Win32_Process -Filter "ProcessId = $($connection.OwningProcess)"
        if (!$process -or ![string]::Equals($process.ExecutablePath, $executable, [StringComparison]::OrdinalIgnoreCase)) {
            throw "端口 $port 已由其他程序使用；未停止或覆盖已有服务。"
        }
    }
    return $true
}

function Wait-Listener([int]$port, [string]$executable) {
    for ($attempt = 0; $attempt -lt 40; $attempt++) {
        if (Test-ExpectedListener $port $executable) { return }
        Start-Sleep -Milliseconds 500
    }
    throw "服务未在端口 $port 就绪，请查看 .runtime 中的日志。"
}

if (!(Test-ExpectedListener $config.databasePort $database)) {
    $dbConfig = (Join-Path $runtime 'db\my.ini').Replace('\', '/')
    Start-Process -FilePath $database -ArgumentList "--defaults-file=`"$dbConfig`"", '--bind-address=127.0.0.1', '--console' `
        -WorkingDirectory $runtime -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $runtime 'mariadb.stdout.log') `
        -RedirectStandardError (Join-Path $runtime 'mariadb.stderr.log') | Out-Null
}
Wait-Listener $config.databasePort $database
if (!(Test-ExpectedListener $config.port $server)) {
    Start-Process -FilePath $server -ArgumentList 'web', '--host', '127.0.0.1', '--port', $config.port, '--env', 'prod' `
        -WorkingDirectory $runtime -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $runtime 'web.stdout.log') `
        -RedirectStandardError (Join-Path $runtime 'web.stderr.log') | Out-Null
}
Wait-Listener $config.port $server
Write-Output "Go 服务已运行：http://127.0.0.1:$($config.port)/#/task"
Write-Output '数据保存在 .runtime\db，初始管理员登录信息见 .runtime\ADMIN.txt；请在首次登录后修改密码。'
