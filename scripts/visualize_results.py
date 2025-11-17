#!/usr/bin/env python3
"""
Visualize load test results for distributed KV store.
Generates graphs showing latency distributions and read-write intervals.
"""

import json
import sys
import glob
import os
from pathlib import Path
import matplotlib.pyplot as plt
import numpy as np
from typing import Dict, List, Any

def load_results(filename: str) -> Dict[str, Any]:
    """Load results from JSON file."""
    with open(filename, 'r') as f:
        return json.load(f)

def plot_latency_distribution(data: List[float], title: str, output_file: str, color: str = 'blue'):
    """
    Plot latency distribution with histogram and percentile markers.
    Shows the long tail clearly.
    """
    if not data:
        print(f"No data for {title}")
        return
    
    fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(12, 10))
    
    # Convert to milliseconds
    data_ms = [x * 1000 for x in data]
    
    # Histogram
    ax1.hist(data_ms, bins=50, color=color, alpha=0.7, edgecolor='black')
    ax1.set_xlabel('Latency (ms)', fontsize=12)
    ax1.set_ylabel('Frequency', fontsize=12)
    ax1.set_title(f'{title} - Histogram', fontsize=14, fontweight='bold')
    ax1.grid(True, alpha=0.3)
    
    # Add percentile lines
    p50 = np.percentile(data_ms, 50)
    p95 = np.percentile(data_ms, 95)
    p99 = np.percentile(data_ms, 99)
    
    ax1.axvline(p50, color='green', linestyle='--', linewidth=2, label=f'P50: {p50:.2f}ms')
    ax1.axvline(p95, color='orange', linestyle='--', linewidth=2, label=f'P95: {p95:.2f}ms')
    ax1.axvline(p99, color='red', linestyle='--', linewidth=2, label=f'P99: {p99:.2f}ms')
    ax1.legend(fontsize=10)
    
    # CDF (Cumulative Distribution Function) - shows long tail clearly
    sorted_data = np.sort(data_ms)
    cdf = np.arange(1, len(sorted_data) + 1) / len(sorted_data)
    
    ax2.plot(sorted_data, cdf * 100, color=color, linewidth=2)
    ax2.set_xlabel('Latency (ms)', fontsize=12)
    ax2.set_ylabel('Cumulative Percentage (%)', fontsize=12)
    ax2.set_title(f'{title} - Cumulative Distribution (shows long tail)', fontsize=14, fontweight='bold')
    ax2.grid(True, alpha=0.3)
    
    # Mark percentiles on CDF
    ax2.plot(p50, 50, 'go', markersize=10, label=f'P50: {p50:.2f}ms')
    ax2.plot(p95, 95, 'o', color='orange', markersize=10, label=f'P95: {p95:.2f}ms')
    ax2.plot(p99, 99, 'ro', markersize=10, label=f'P99: {p99:.2f}ms')
    ax2.legend(fontsize=10)
    
    # Statistics box
    stats_text = f'Stats:\n'
    stats_text += f'Mean: {np.mean(data_ms):.2f}ms\n'
    stats_text += f'Median: {p50:.2f}ms\n'
    stats_text += f'Std Dev: {np.std(data_ms):.2f}ms\n'
    stats_text += f'Min: {np.min(data_ms):.2f}ms\n'
    stats_text += f'Max: {np.max(data_ms):.2f}ms\n'
    stats_text += f'Count: {len(data_ms)}'
    
    ax2.text(0.98, 0.02, stats_text, transform=ax2.transAxes,
             fontsize=10, verticalalignment='bottom', horizontalalignment='right',
             bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.5))
    
    plt.tight_layout()
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    plt.close()
    print(f"  ✓ Saved: {output_file}")

def plot_interval_distribution(data: List[float], title: str, output_file: str):
    """Plot distribution of read-write intervals."""
    if not data:
        print(f"No data for {title}")
        return
    
    fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(12, 10))
    
    # Convert to milliseconds
    data_ms = [x * 1000 for x in data]
    
    # Histogram
    ax1.hist(data_ms, bins=50, color='purple', alpha=0.7, edgecolor='black')
    ax1.set_xlabel('Interval (ms)', fontsize=12)
    ax1.set_ylabel('Frequency', fontsize=12)
    ax1.set_title(f'{title} - Histogram', fontsize=14, fontweight='bold')
    ax1.grid(True, alpha=0.3)
    
    # Add statistics
    mean_val = np.mean(data_ms)
    median_val = np.median(data_ms)
    
    ax1.axvline(mean_val, color='green', linestyle='--', linewidth=2, label=f'Mean: {mean_val:.2f}ms')
    ax1.axvline(median_val, color='orange', linestyle='--', linewidth=2, label=f'Median: {median_val:.2f}ms')
    ax1.legend(fontsize=10)
    
    # CDF
    sorted_data = np.sort(data_ms)
    cdf = np.arange(1, len(sorted_data) + 1) / len(sorted_data)
    
    ax2.plot(sorted_data, cdf * 100, color='purple', linewidth=2)
    ax2.set_xlabel('Interval (ms)', fontsize=12)
    ax2.set_ylabel('Cumulative Percentage (%)', fontsize=12)
    ax2.set_title(f'{title} - Cumulative Distribution', fontsize=14, fontweight='bold')
    ax2.grid(True, alpha=0.3)
    
    # Statistics box
    stats_text = f'Stats:\n'
    stats_text += f'Mean: {mean_val:.2f}ms\n'
    stats_text += f'Median: {median_val:.2f}ms\n'
    stats_text += f'Std Dev: {np.std(data_ms):.2f}ms\n'
    stats_text += f'Min: {np.min(data_ms):.2f}ms\n'
    stats_text += f'Max: {np.max(data_ms):.2f}ms\n'
    stats_text += f'Count: {len(data_ms)}'
    
    ax2.text(0.98, 0.02, stats_text, transform=ax2.transAxes,
             fontsize=10, verticalalignment='bottom', horizontalalignment='right',
             bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.5))
    
    plt.tight_layout()
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    plt.close()
    print(f"  ✓ Saved: {output_file}")

def plot_comparison(results_files: List[str], output_dir: str):
    """Create comparison plots across different configurations."""
    configs = []
    read_p99 = []
    write_p99 = []
    stale_rates = []
    
    for filename in results_files:
        try:
            data = load_results(filename)
            config_name = Path(filename).stem.replace('results_', '')
            
            configs.append(config_name)
            
            if data['read_latencies']:
                read_p99.append(np.percentile(data['read_latencies'], 99) * 1000)
            else:
                read_p99.append(0)
            
            if data['write_latencies']:
                write_p99.append(np.percentile(data['write_latencies'], 99) * 1000)
            else:
                write_p99.append(0)
            
            total_reads = len(data['read_latencies']) if data['read_latencies'] else 0
            stale_reads_count = len(data['stale_reads']) if data['stale_reads'] else 0
            if total_reads > 0:
                stale_rate = (stale_reads_count / total_reads) * 100
                stale_rates.append(stale_rate)
            else:
                stale_rates.append(0)
        except Exception as e:
            print(f"Error processing {filename}: {e}")
            continue
    
    if not configs:
        return
    
    # P99 Latency Comparison
    fig, ax = plt.subplots(figsize=(14, 8))
    x = np.arange(len(configs))
    width = 0.35
    
    bars1 = ax.bar(x - width/2, read_p99, width, label='Read P99', color='skyblue', edgecolor='black')
    bars2 = ax.bar(x + width/2, write_p99, width, label='Write P99', color='lightcoral', edgecolor='black')
    
    ax.set_xlabel('Configuration', fontsize=12, fontweight='bold')
    ax.set_ylabel('P99 Latency (ms)', fontsize=12, fontweight='bold')
    ax.set_title('P99 Latency Comparison Across Configurations', fontsize=14, fontweight='bold')
    ax.set_xticks(x)
    ax.set_xticklabels(configs, rotation=45, ha='right')
    ax.legend(fontsize=10)
    ax.grid(True, alpha=0.3, axis='y')
    
    # Add value labels on bars
    for bars in [bars1, bars2]:
        for bar in bars:
            height = bar.get_height()
            ax.text(bar.get_x() + bar.get_width()/2., height,
                   f'{height:.1f}',
                   ha='center', va='bottom', fontsize=8)
    
    plt.tight_layout()
    output_file = os.path.join(output_dir, 'comparison_p99_latency.png')
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    plt.close()
    print(f"  ✓ Saved: {output_file}")
    
    # Stale Read Rate Comparison
    fig, ax = plt.subplots(figsize=(14, 8))
    bars = ax.bar(configs, stale_rates, color='orange', edgecolor='black', alpha=0.7)
    
    ax.set_xlabel('Configuration', fontsize=12, fontweight='bold')
    ax.set_ylabel('Stale Read Rate (%)', fontsize=12, fontweight='bold')
    ax.set_title('Stale Read Rate Comparison Across Configurations', fontsize=14, fontweight='bold')
    plt.xticks(rotation=45, ha='right')
    ax.grid(True, alpha=0.3, axis='y')
    
    # Add value labels on bars
    for bar in bars:
        height = bar.get_height()
        ax.text(bar.get_x() + bar.get_width()/2., height,
               f'{height:.2f}%',
               ha='center', va='bottom', fontsize=9)
    
    plt.tight_layout()
    output_file = os.path.join(output_dir, 'comparison_stale_reads.png')
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    plt.close()
    print(f"  ✓ Saved: {output_file}")

def generate_summary_report(results_files: List[str], output_file: str):
    """Generate a text summary report of all results."""
    with open(output_file, 'w') as f:
        f.write("=" * 80 + "\n")
        f.write("DISTRIBUTED KV STORE - LOAD TEST SUMMARY REPORT\n")
        f.write("=" * 80 + "\n\n")
        
        for filename in results_files:
            try:
                data = load_results(filename)
                config_name = Path(filename).stem.replace('results_', '')
                
                f.write(f"\n{'=' * 80}\n")
                f.write(f"Configuration: {config_name}\n")
                f.write(f"{'=' * 80}\n\n")
                
                f.write(f"Mode: {data.get('mode', 'N/A')}\n")
                f.write(f"Total Writes: {len(data['write_latencies'])}\n")
                f.write(f"Total Reads: {len(data['read_latencies'])}\n\n")
                
                # Write statistics
                if data['write_latencies']:
                    write_ms = [x * 1000 for x in data['write_latencies']]
                    f.write("Write Latency:\n")
                    f.write(f"  Mean:   {np.mean(write_ms):.2f} ms\n")
                    f.write(f"  Median: {np.median(write_ms):.2f} ms\n")
                    f.write(f"  P95:    {np.percentile(write_ms, 95):.2f} ms\n")
                    f.write(f"  P99:    {np.percentile(write_ms, 99):.2f} ms\n")
                    f.write(f"  Max:    {np.max(write_ms):.2f} ms\n\n")
                
                # Read statistics
                if data['read_latencies']:
                    read_ms = [x * 1000 for x in data['read_latencies']]
                    f.write("Read Latency:\n")
                    f.write(f"  Mean:   {np.mean(read_ms):.2f} ms\n")
                    f.write(f"  Median: {np.median(read_ms):.2f} ms\n")
                    f.write(f"  P95:    {np.percentile(read_ms, 95):.2f} ms\n")
                    f.write(f"  P99:    {np.percentile(read_ms, 99):.2f} ms\n")
                    f.write(f"  Max:    {np.max(read_ms):.2f} ms\n\n")
                
                # Stale reads
                total_reads = len(data['read_latencies'])
                stale_count = len(data['stale_reads'])
                if total_reads > 0:
                    stale_rate = (stale_count / total_reads) * 100
                    f.write(f"Stale Reads: {stale_count} ({stale_rate:.2f}%)\n\n")
                
                # Read-write intervals
                if data['read_write_intervals']:
                    interval_ms = [x * 1000 for x in data['read_write_intervals']]
                    f.write("Read-Write Intervals:\n")
                    f.write(f"  Mean:   {np.mean(interval_ms):.2f} ms\n")
                    f.write(f"  Median: {np.median(interval_ms):.2f} ms\n")
                    f.write(f"  Min:    {np.min(interval_ms):.2f} ms\n")
                    f.write(f"  Max:    {np.max(interval_ms):.2f} ms\n\n")
                
            except Exception as e:
                f.write(f"\nError processing {filename}: {e}\n\n")
    
    print(f"  ✓ Saved: {output_file}")

def main():
    if len(sys.argv) > 1:
        # Process specific files
        results_files = sys.argv[1:]
    else:
        # Find all result files in results directory
        results_files = glob.glob("results/*.json")
        # Fallback to current directory if results folder doesn't exist
        if not results_files:
            results_files = glob.glob("*.json")
    
    if not results_files:
        print("No result files found. Please run load tests first or specify result files.")
        print("Expected location: results/*.json")
        sys.exit(1)
    
    # Create output directory
    output_dir = "visualizations"
    os.makedirs(output_dir, exist_ok=True)
    
    print(f"\n{'=' * 60}")
    print("Generating visualizations for load test results")
    print(f"{'=' * 60}\n")
    print(f"Found {len(results_files)} result file(s)")
    print(f"Output directory: {output_dir}\n")
    
    # Process each result file
    for filename in results_files:
        try:
            print(f"Processing: {filename}")
            data = load_results(filename)
            
            base_name = Path(filename).stem
            
            # Plot read latency
            if data['read_latencies']:
                plot_latency_distribution(
                    data['read_latencies'],
                    f"{base_name} - Read Latency Distribution",
                    os.path.join(output_dir, f"{base_name}_read_latency.png"),
                    color='steelblue'
                )
            
            # Plot write latency
            if data['write_latencies']:
                plot_latency_distribution(
                    data['write_latencies'],
                    f"{base_name} - Write Latency Distribution",
                    os.path.join(output_dir, f"{base_name}_write_latency.png"),
                    color='coral'
                )
            
            # Plot read-write intervals
            if data['read_write_intervals']:
                plot_interval_distribution(
                    data['read_write_intervals'],
                    f"{base_name} - Read-Write Interval Distribution",
                    os.path.join(output_dir, f"{base_name}_intervals.png")
                )
            
            print()
        except Exception as e:
            print(f"Error processing {filename}: {e}\n")
    
    # Generate comparison plots
    if len(results_files) > 1:
        print("Generating comparison plots...")
        plot_comparison(results_files, output_dir)
        print()
    
    # Generate summary report
    print("Generating summary report...")
    generate_summary_report(results_files, os.path.join(output_dir, "summary_report.txt"))
    
    print(f"\n{'=' * 60}")
    print("✓ All visualizations generated successfully!")
    print(f"{'=' * 60}\n")
    print(f"Check the '{output_dir}' directory for all graphs and reports.\n")

if __name__ == "__main__":
    main()

