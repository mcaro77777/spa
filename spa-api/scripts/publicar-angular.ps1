param(
    [string]$Origen = "..\spa-web\dist\spa-web\browser",
    [string]$Destino = ".\ClientApp"
)

$ErrorActionPreference = 'Stop'
$raizApi = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$rutaOrigen = [IO.Path]::GetFullPath((Join-Path $raizApi $Origen))
$rutaDestino = [IO.Path]::GetFullPath((Join-Path $raizApi $Destino))
$destinoPermitido = [IO.Path]::GetFullPath((Join-Path $raizApi 'ClientApp'))

if ($rutaDestino -ne $destinoPermitido) {
    throw "El destino debe ser exactamente $destinoPermitido"
}
if (-not (Test-Path -LiteralPath (Join-Path $rutaOrigen 'index.html'))) {
    throw "No se encontro un build Angular valido en $rutaOrigen"
}

Get-ChildItem -LiteralPath $rutaDestino -Force -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -ne '.gitkeep' } |
    Remove-Item -Recurse -Force
Copy-Item -Path (Join-Path $rutaOrigen '*') -Destination $rutaDestino -Recurse -Force

Write-Host "Angular publicado en $rutaDestino"

