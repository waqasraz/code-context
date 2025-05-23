# Code Context

A CLI tool that scans a codebase, analyzes the code using an LLM based on a user-provided query/topic, and produces a structured Markdown summary file.

## Overview

Code Context helps developers quickly understand unfamiliar codebases by leveraging Large Language Models (LLMs) to analyze and summarize code based on specific queries. It can:

- Scan a directory or set of subdirectories
- Filter and identify relevant files based on your query
- Generate summaries of the most relevant files using an LLM
- Optionally create a visual directory tree
- Compile everything into a single Markdown file for easy reference

## Installation

### Requirements

- Go 1.20 or higher

### Building from Source

```bash
# Clone the repository
git clone https://github.com/waqasraz/code-context.git
cd code-context

# Build the project
go build

# Optionally, install the binary to your Go bin directory
go install
```

## Usage

Basic usage:

```bash
code-context [options] TARGET_PATH QUERY
```

Where:
- `TARGET_PATH` is the path to the directory to analyze
- `QUERY` is the natural language query defining the context to search for

### Options

- `-o, --output <FILENAME>`: Specify the output Markdown file name (default: `CODE_CONTEXT_SUMMARY.md`)
- `--llm-api-key <KEY>`: API key for the LLM service (required for OpenAI, Gemini, Anthropic, but not for local providers like Ollama).
- `--llm-endpoint <URL>`: Endpoint for the LLM service (or use LLM_ENDPOINT env var).
- `--llm-provider <PROVIDER>`: LLM provider to use: 'openai', 'local', 'unified', or empty for placeholder.
- `--llm-model <MODEL>`: Model name to use with the LLM provider.
- `--llm-header <KEY:VALUE>`: Additional headers for LLM API requests (repeatable, format: 'key:value').
- `--ignore <PATTERN>`: Glob patterns for files/directories to ignore (repeatable).
- `--show-tree`: Include a directory tree structure in the output.
- `--use-embeddings`: Use embedding-based relevance detection for more accurate results.
- `--use-hybrid`: Use hybrid approach combining embeddings with keywords and path relevance (default: true).
- `--no-hybrid`: Disable hybrid relevance detection and use pure embeddings or keywords.
- `--embedding-provider <PROVIDER>`: Embedding provider: 'ollama', 'gemini', 'openai' (soon), 'anthropic' (soon). Default: 'ollama'.
- `--embedding-model <MODEL>`: Model to use for embeddings (e.g., "nomic-embed-text", "gemini-embedding-001"). Default: "nomic-embed-text".
- `--embedding-api-key <KEY>`: API key for the embedding model, if different from LLM API key (required for Gemini, OpenAI, Anthropic, but not for Ollama).
- `--embedding-endpoint <URL>`: Endpoint URL for embedding API (used for 'ollama'/'local' provider). Default: "http://localhost:11434/api/embeddings".

### Environment Variables

Instead of passing LLM configuration flags, you can set the following environment variables:

- `LLM_API_KEY`: API key for the LLM service
- `LLM_ENDPOINT`: Endpoint for the LLM service
- `LLM_PROVIDER`: LLM provider to use
- `LLM_MODEL`: Model name to use

## Examples

### Analyze a single service/directory for Kafka usage

```bash
code-context ./my-service/ "Explain the Kafka integration points"
```

### Analyze multiple directories with the PowerShell script

```powershell
# Process all subdirectories in a parent directory
.\run-multiple-dirs.ps1 -QueryString "general working of service and different components" -ParentDirectory "C:\Users\waqas\Documents\OneStop2.0"

# With custom LLM settings
.\run-multiple-dirs.ps1 -QueryString "Find authentication flows" -ParentDirectory ".\projects\" -LLMProvider gemini -LLMModel "gemini-pro" -ApiKey "your-api-key"

```

### Specify output file name

```bash
code-context ./project-x/ "Document the main API endpoints" -o API_Endpoints.md
```

### Include a directory tree in the output

```bash
code-context ./complex-project/ "Identify database models" --show-tree
```

### Use OpenAI API for summaries

```bash
code-context ./my-project/ "Explain the authentication flow" \
  --llm-provider openai \
  --llm-model gpt-4 \
  --llm-api-key your-api-key-here
```

### Use Unified API for multiple LLM providers

```bash
code-context ./my-project/ "Explain the user authentication flow" \
  --llm-provider unified \
  --llm-endpoint "https://api.litellm.ai/v1/chat/completions" \
  --llm-model "gpt-4" \
  --llm-header "x-api-version:v1" \
  --llm-header "x-provider:anthropic"
```

### Ignore specific directories

```bash
code-context ./my-project/ "Find all HTTP endpoints" \
  --ignore "**/test/**" --ignore "**/docs/**"
```

### Use embedding-based relevance detection

Use AI embeddings to find semantically relevant files, providing more accurate results than keyword matching.

```bash
# Example using default Ollama provider
code-context ./my-project/ "Explain the authentication flow" \
  --use-embeddings \
  --embedding-model nomic-embed-text \
  --embedding-endpoint http://localhost:11434/api/embeddings # Optional if using default

# Example using Gemini Embedding Model (requires API Key)
code-context ./my-project/ "Explain the authentication flow" \
  --use-embeddings \
  --embedding-provider gemini \
  --embedding-model gemini-embedding-001 \
  --embedding-api-key your-embedding-api-key \
  --llm-provider openai \
  --llm-model gpt-4 \
  --llm-api-key your-openai-api-key

# Example using OpenAI (coming soon)
# code-context ./my-project/ "Explain the authentication flow" \
#   --use-embeddings \
#   --embedding-provider openai \
#   --embedding-model text-embedding-ada-002 \
#   --llm-api-key your-openai-api-key
```

### Use hybrid relevance detection (recommended)

Combines the power of embeddings with traditional keyword matching and path relevance for optimal results.

```bash
# Example using default Ollama provider with Hybrid Search
code-context ./my-project/ "Explain the authentication flow" \
  --use-hybrid \
  --embedding-model nomic-embed-text

# Example using Gemini Embedding Model with Hybrid Search and separate API keys
code-context ./my-project/ "Explain the authentication flow" \
  --use-hybrid \
  --embedding-provider gemini \
  --embedding-model gemini-embedding-001 \
  --embedding-api-key your-gemini-api-key \
  --llm-provider anthropic \
  --llm-model claude-3-sonnet \
  --llm-api-key your-anthropic-api-key
```

## LLM Integration

Code Context supports multiple LLM providers:

1. **OpenAI** (`--llm-provider openai`): Uses OpenAI's API for generating summaries
2. **Anthropic** (`--llm-provider anthropic`): Uses Anthropic's Claude models
3. **Google Gemini** (`--llm-provider gemini`): Uses Google's Gemini models
4. **Local** (`--llm-provider local`): Connects to locally hosted LLM APIs, including:
   - **Ollama**: Run models like Llama, Mistral, or others locally
   - Other local self-hosted APIs with compatible formats
5. **Unified** (`--llm-provider unified`): Uses a unified API that can route to multiple LLM providers
6. **Placeholder** (default when no provider specified): Generates basic file statistics without using an LLM

### Examples

#### Using OpenAI

```bash
code-context ./my-project/ "Explain the authentication flow" \
  --llm-provider openai \
  --llm-model gpt-4 \
  --llm-api-key your-openai-api-key
```

#### Using Anthropic Claude

```bash
code-context ./my-project/ "Explain the authentication flow" \
  --llm-provider anthropic \
  --llm-model claude-3-opus-20240229 \
  --llm-api-key your-anthropic-api-key
```

#### Using Google Gemini

```bash
code-context ./my-project/ "Explain the authentication flow" \
  --llm-provider gemini \
  --llm-model gemini-pro \
  --llm-api-key your-google-api-key
```

#### Using Ollama locally

```bash
# First, make sure Ollama is running locally
# Then run:
code-context ./my-project/ "Explain the authentication flow" \
  --llm-provider local \
  --llm-model llama2 \
  --llm-endpoint "http://localhost:11434/api/generate"
```

### Unified API for Multiple LLM Providers

The unified adapter allows you to use various LLM services through a single standardized interface. This is especially useful when you want to:

- Switch between different LLM providers without changing your code
- Use specialized routing services like LiteLLM
- Connect to your organization's internal LLM gateway

When using the unified provider, you can pass additional headers to customize the request using the `--llm-header` flag.

### Configuration

The tool prioritizes LLM configuration in this order:
1. Command-line flags
2. Environment variables
3. Default placeholder provider

## Output Format

The generated Markdown file contains:

1. A header with the query and target directory
2. An optional directory tree showing the structure (with relevant files marked)
3. File summaries organized by relevance to the query
4. In multi-service mode, summaries grouped by subdirectory

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the terms of the included LICENSE file. 