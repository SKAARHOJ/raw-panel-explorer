# build-and-zip.ps1

# Step 1: Run the build command
Write-Host "Running wails build..."
wails build -tags runningwails

# Check if build succeeded
if ($LASTEXITCODE -ne 0) {
    Write-Error "wails build failed. Exiting script."
    exit 1
}

# Step 2: Define paths
$inputFile = "build/bin/Raw Panel Explorer.exe"
$zipFile = "binaries/Raw Panel Explorer.exe.zip"
$zipDir = Split-Path -Parent $zipFile

# Ensure the output directory exists
if (-not (Test-Path $zipDir)) {
    New-Item -ItemType Directory -Path $zipDir -Force
}

# Step 3: Create zip
Write-Host "Zipping the output file..."
Compress-Archive -Path $inputFile -DestinationPath $zipFile -Force

Write-Host "Done. Created zip file at: $zipFile"
