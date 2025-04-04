# PowerShell script to run code-context on multiple directories
param(
    [Parameter(Mandatory=$true)]
    [string]$QueryString,
    
    [Parameter(Mandatory=$true)]
    [string]$ParentDirectory,
    
    [string]$LLMProvider = "local",
    
    [string]$LLMModel = "llama3.2",
    
    [string]$EmbeddingProvider = "ollama",
    
    [string]$EmbeddingModel = "llama3.2",
    
    [string]$ApiKey = "",
    
    [string[]]$DirectoriesToSkip = @(".idea", ".git", "node_modules")
)

# Verify parent directory exists
if (-not (Test-Path -Path $ParentDirectory)) {
    Write-Error "Parent directory '$ParentDirectory' does not exist."
    exit 1
}

# Get all immediate subdirectories
$directories = Get-ChildItem -Path $ParentDirectory -Directory | 
    Where-Object { $DirectoriesToSkip -notcontains $_.Name } |
    Select-Object -ExpandProperty FullName

# Display info
Write-Host "Will process these directories with query: '$QueryString'"
Write-Host "LLM: $LLMProvider / $LLMModel"
Write-Host "Embedding: $EmbeddingProvider / $EmbeddingModel"
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
    
    # Execute the command
    Write-Host "Executing: $command"
    Invoke-Expression $command
    
    Write-Host "`nFinished processing: $dirName`n"
}

Write-Host "All directories processed successfully!" -ForegroundColor Green 