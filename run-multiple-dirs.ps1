# PowerShell script to run code-context on multiple directories
param(
    [Parameter(Mandatory=$true)]
    [string]$QueryString,
    
    [Parameter(Mandatory=$true)]
    [string]$ParentDirectory,
    
    [string]$LLMProvider = "local",
    
    [string]$LLMModel = "gemma3:12b",
    
    [string]$EmbeddingProvider = "ollama",
    
    [string]$EmbeddingModel = "llama3.2",
    
    [string]$ApiKey = "",
    
    [string]$EmbeddingApiKey = "",
    
    [string]$EmbeddingEndpoint = "",
    
    [string[]]$DirectoriesToSkip = @(".idea", ".git", "node_modules"),
    
    [string[]]$SpecificFolders = @(),
    
    [switch]$Interactive
)

# Verify parent directory exists
if (-not (Test-Path -Path $ParentDirectory)) {
    Write-Error "Parent directory '$ParentDirectory' does not exist."
    exit 1
}

# Get all available subdirectories
$availableDirectories = Get-ChildItem -Path $ParentDirectory -Directory | 
    Where-Object { $DirectoriesToSkip -notcontains $_.Name } |
    Select-Object -ExpandProperty FullName

# If neither SpecificFolders nor Interactive is specified, process all directories
if ($SpecificFolders.Count -eq 0 -and -not $Interactive) {
    $directories = $availableDirectories
}
# If specific folders are provided, filter to only those directories
elseif ($SpecificFolders.Count -gt 0) {
    $directories = $availableDirectories | Where-Object {
        $dirName = Split-Path -Path $_ -Leaf
        $SpecificFolders -contains $dirName
    }
    
    # Check if any specified folders were not found
    $foundDirNames = $directories | ForEach-Object { Split-Path -Path $_ -Leaf }
    $notFound = $SpecificFolders | Where-Object { $foundDirNames -notcontains $_ }
    if ($notFound.Count -gt 0) {
        Write-Warning "The following specified folders were not found: $($notFound -join ', ')"
    }
}
# If interactive mode is enabled, let the user select directories
else {
    $dirOptions = @()
    $index = 1
    $availableDirectories | ForEach-Object {
        $dirName = Split-Path -Path $_ -Leaf
        $dirOptions += [PSCustomObject]@{
            Index = $index
            Name = $dirName
            Path = $_
        }
        $index++
    }
    
    # Display available directories
    Write-Host "Available directories:" -ForegroundColor Cyan
    $dirOptions | ForEach-Object {
        Write-Host "  [$($_.Index)] $($_.Name)"
    }
    
    # Get user selection
    Write-Host "`nEnter directory numbers to process (comma-separated) or 'all' for all directories:"
    $selection = Read-Host
    
    if ($selection -eq "all") {
        $directories = $availableDirectories
    }
    else {
        $selectedIndices = $selection -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ -match '^\d+$' } | ForEach-Object { [int]$_ }
        $directories = $dirOptions | Where-Object { $selectedIndices -contains $_.Index } | Select-Object -ExpandProperty Path
        
        if ($directories.Count -eq 0) {
            Write-Error "No valid directories selected. Exiting."
            exit 1
        }
    }
}

# Display info
Write-Host "Will process these directories with query: '$QueryString'"
Write-Host "LLM: $LLMProvider / $LLMModel"
Write-Host "Embedding: $EmbeddingProvider / $EmbeddingModel"
if ($EmbeddingEndpoint) {
    Write-Host "Embedding Endpoint: $EmbeddingEndpoint"
}
Write-Host "Directories to process:" -ForegroundColor Cyan
$directories | ForEach-Object { Write-Host "  $_" }

# Ask for confirmation
$confirmation = Read-Host "Continue? (y/n)"
if ($confirmation -ne 'y') {
    Write-Host "Operation cancelled."
    exit
}

# Process each directory
foreach ($dir in $directories) {
    $dirName = Split-Path -Path $dir -Leaf
    Write-Host "`n=========================================" -ForegroundColor Green
    Write-Host "Processing directory: $dirName" -ForegroundColor Green
    Write-Host "=========================================`n" -ForegroundColor Green
    
    # Build the command based on API key presence
    $command = ".\code-context.exe `"$dir`" `"$QueryString`" --llm-provider=$LLMProvider --llm-model=$LLMModel --embedding-provider=$EmbeddingProvider --embedding-model=$EmbeddingModel"
    
    if ($ApiKey -ne "") {
        $command += " --llm-api-key=$ApiKey"
    }
    
    if ($EmbeddingApiKey -ne "") {
        $command += " --embedding-api-key=$EmbeddingApiKey"
    }
    
    if ($EmbeddingEndpoint -ne "") {
        $command += " --embedding-endpoint=$EmbeddingEndpoint"
    }
    
    # Execute the command
    Write-Host "Executing: $command"
    Invoke-Expression $command
    
    Write-Host "`nFinished processing: $dirName`n"
}

Write-Host "All directories processed successfully!" -ForegroundColor Green 


