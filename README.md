# Podcast Statistics Analyzer

A Go program that analyzes podcast RSS/Atom feeds from an OPML file and generates statistics and histograms. Now available with both CLI and TUI (Terminal User Interface) modes!

## Features

- **Two Interface Modes:**
  - **TUI Mode**: Interactive terminal interface with file picker, configuration, and editable results
  - **CLI Mode**: Traditional command-line interface for scripting and automation
- Parses OPML files to extract podcast feed URLs
- Fetches and analyzes RSS/Atom feeds
- Calculates multiple statistics per podcast:
  - Number of unlistened episodes
  - Average episode length in minutes (adjusted for playback speed)
  - Average days between episodes
  - Days since the most recent episode
- Combines metrics into a composite score
- Displays results in a 10-bucket histogram
- Caches user input for faster subsequent runs
- Supports Value-for-Value podcasts (marked with ⚡)

## Usage

### TUI Mode (Recommended)

```bash
# Run the interactive TUI
./podstats --tui
```

The TUI provides:

- 📁 Interactive file picker for OPML files
- ⚙️ Configuration screen for cache options  
- 🔄 Real-time processing with progress indicator
- 📊 Sortable results list with rankings
- ✏️ Edit individual podcast settings (unlistened count, playback speed)
- 💾 Automatic cache management

### CLI Mode

```bash
# Build the program
go build -o podstats main.go

# Run with an OPML file
./podstats your_podcasts.opml
```

## OPML File Format

The program expects a standard OPML file with podcast subscriptions. Each podcast should have an `xmlUrl` attribute pointing to the RSS/Atom feed.

Example OPML structure:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
  <head>
    <title>My Podcast Subscriptions</title>
  </head>
  <body>
    <outline text="Technology">
      <outline text="The Changelog" 
               title="The Changelog" 
               type="rss" 
               xmlUrl="https://changelog.com/podcast/feed" 
               htmlUrl="https://changelog.com/podcast"/>
    </outline>
  </body>
</opml>
```

## Statistics Explanation

### Individual Metrics
- **Unlistened Episodes**: Total number of episodes in the feed
- **Average Episode Length**: Mean duration in minutes (defaults to 30 if not available)
- **Average Days Between Episodes**: Mean time between episode releases
- **Days Since Latest**: Time since the most recent episode was published

### Composite Score
The composite score combines all metrics into a single value (0.0 to 1.0):
- **Lower scores are better** (represent more manageable podcasts)
- Weighted formula:
  - Unlistened episodes: 40%
  - Episode length: 20%
  - Publication frequency: 20%
  - Recency: 20%

### Histogram
The final histogram shows the distribution of composite scores across all analyzed podcasts, divided into 10 equally-sized buckets.

## Sample Output

```
Parsing OPML file: sample.opml
Found 4 podcasts in OPML file

[1/4] Analyzing: The Changelog
  URL: https://changelog.com/podcast/feed
  Episodes: 918 unlistened
  Avg length: 62.9 minutes
  Avg days between: 6.2
  Days since latest: 2.7
  Composite score: 0.55

=== PODCAST STATISTICS HISTOGRAM ===

Composite Score Distribution (4 podcasts):

0.0-0.1 | 0
0.1-0.2 | 0
0.2-0.3 | 0
0.3-0.4 |████████████████████████████████████████████████ 1
0.4-0.5 |████████████████████████████████████████████████ 1
0.5-0.6 |████████████████████████████████████████████████ 1
0.6-0.7 |████████████████████████████████████████████████ 1
0.7-0.8 | 0
0.8-0.9 | 0
0.9-1.0 | 0
```

## Dependencies

- Go 1.21 or later
- No external dependencies (uses only standard library)

## Error Handling

The program handles various error conditions:
- Invalid OPML files
- Unreachable RSS/Atom feeds
- Malformed feed content
- Missing or invalid date formats
- Network timeouts

Failed feeds are skipped with error messages, and the analysis continues with the remaining podcasts.
