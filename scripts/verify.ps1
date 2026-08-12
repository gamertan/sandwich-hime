# SPDX-License-Identifier: AGPL-3.0-only

[CmdletBinding()]
param(
    [switch]$Race
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $RepoRoot

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Label,
        [Parameter(Mandatory = $true)]
        [scriptblock]$Command
    )

    Write-Host "`n==> $Label"
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Label failed with exit code $LASTEXITCODE"
    }
}

function Invoke-ModuleChecks {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Directory,
        [Parameter(Mandatory = $true)]
        [string]$Label
    )

    Push-Location $Directory
    try {
        Invoke-Checked "$Label`: go test" { go test ./... }
        Invoke-Checked "$Label`: go vet" { go vet ./... }
    }
    finally {
        Pop-Location
    }
}

function Get-SandoSources {
    if (-not (Test-Path "internal/compiler/testdata/golden" -PathType Container)) {
        return @()
    }

    return @(Get-ChildItem "internal/compiler/testdata/golden" -File -Filter "*.sando" |
        Sort-Object FullName)
}

function Get-GeneratedManifest {
    $lines = foreach ($source in (Get-SandoSources)) {
        $output = "$($source.FullName).go"
        if (-not (Test-Path $output -PathType Leaf)) {
            "missing  $output"
            continue
        }
        $hash = (Get-FileHash -Algorithm SHA256 $output).Hash.ToLowerInvariant()
        $modified = (Get-Item -LiteralPath $output).LastWriteTimeUtc.Ticks
        "$hash  $modified  $output"
    }
    return ($lines -join "`n")
}

$TempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("himesan-verify-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $TempRoot | Out-Null

try {
    Invoke-ModuleChecks "." "compiler module"
    Invoke-Checked "compiler module: go build" {
        go build -trimpath -o (Join-Path $TempRoot "himesan.exe") ./cmd/himesan
    }

    if (-not (Test-Path "sando/go.mod" -PathType Leaf)) {
        throw "nested Apache runtime module sando/go.mod is missing"
    }
    Invoke-ModuleChecks "sando" "sando runtime module"

    $Sources = @(Get-SandoSources)
    if ($Sources.Count -eq 0) {
        throw "compiler-owned golden .sando fixture is missing"
    }
    else {
        $SourcePaths = @($Sources | ForEach-Object { $_.FullName })
        $CheckArgs = @("run", "./cmd/himesan", "check") + $SourcePaths
        $GenerateArgs = @("run", "./cmd/himesan", "generate") + $SourcePaths
        Invoke-Checked "golden generation: read-only freshness check" {
            & go $CheckArgs
        }
        $Before = Get-GeneratedManifest

        Invoke-Checked "golden generation: first deterministic pass" {
            & go $GenerateArgs
        }
        $First = Get-GeneratedManifest
        if ($Before -cne $First) {
            throw "generation changed committed output after check declared it fresh"
        }

        Invoke-Checked "golden generation: second deterministic pass" {
            & go $GenerateArgs
        }
        $Second = Get-GeneratedManifest
        if ($First -cne $Second) {
            throw "repeated generation changed output bytes or an unchanged timestamp"
        }

        Invoke-Checked "golden generation: final freshness check" {
            & go $CheckArgs
        }
    }

    if ($Race) {
        Invoke-Checked "compiler module: race tests" { go test -race ./... }
        Push-Location "sando"
        try {
            Invoke-Checked "sando runtime module: race tests" { go test -race ./... }
        }
        finally {
            Pop-Location
        }
    }

    Write-Host "`n==> verification complete"
}
finally {
    if (Test-Path $TempRoot -PathType Container) {
        Remove-Item -LiteralPath $TempRoot -Recurse -Force
    }
}
