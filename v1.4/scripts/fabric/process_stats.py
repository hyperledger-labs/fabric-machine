#!/usr/bin/env python
#
# Copyright Xilinx Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
#
# This program gathers and summarizes peer statistics for a Fabric benchmark/experiment.
#
# Run this utility as:
#   ./process_stats.py -m combinestats-perpeer

import argparse
from datetime import datetime
import csv
import glob
import os
import re
import shutil
import subprocess
import sys
import time
import yaml


# Runs a command and returns its output.
# Arguments:
#   cmd (string): contents of the command
#   num_tries (int): number of times to try in case of failures
#
# Returns:
#   None
def run_cmd(cmd, num_tries):
    print("=== Running cmd ===")
    print(cmd)

    ret = 0
    for i in range(num_tries):
        # Stream the output of the command to console while its running.
        start = time.time()
        process = subprocess.Popen([cmd], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, shell=True)
        while True:
            elapsed = time.time() - start
            if elapsed > 900:  # 15 minutes timeout
                process.kill()
                break

            line = process.stdout.readline()
            if line:
                print(line.strip())
                continue
            if process.poll() is not None:
                break  # break the loop only if line is empty and process has finished

        ret = process.returncode
        if ret == 0:
            break
        else:
            print("Cmd failed ...")
            
    if ret != 0:
        print("Cmd {0} unsuccessful, already tried {1} times and failed ...".format(cmd, i + 1))
        sys.exit(ret)


# Returns the string representation of the input values.
# Arguments:
#   values (list): list of numbers and strings
#   width (int): number of characters to use when converting numbers and strings
#
# Returns:
#   string representation of values
def get_string(values, width=0):
    return ''.join(str(v).zfill(width) for v in values)


# Removes the provided directory.
# Arguments:
#   dir_path (string): path to directory
#
# Returns:
#   None
def remove_dir(dir_path):
    if os.path.exists(dir_path):
        shutil.rmtree(dir_path)


# Creates the provided directory.
# Arguments:
#   dir_path (string): path to directory
#   overwrite (bool): overwrite the directory if it already exists
#
# Returns:
#   None
def create_dir(dir_path, overwrite):
    if overwrite:
        remove_dir(dir_path)
    os.mkdir(dir_path)


# Backups the provided file as filename-backup.extension only if the backup file doesn't
# already exist.
# Arguments:
#   file_path (string): path to file
#
# Returns:
#   None
def backup_file(file_path):
    path_fields = file_path.split('/')
    path_fields[-1] = path_fields[-1].replace('.', '-backup.')
    backup_path = '/'.join(path_fields)
    if os.path.isfile(file_path) and not os.path.isfile(backup_path):
        print('Backing up {0}'.format(file_path))
        os.rename(file_path, backup_path)


# Extracts fabric major version from the provided input.
# For example, it will return 1.4.5 when the input is amd-1.4.5-snapshot-xyz.
# Arguments:
#   version (string): fabric complete version
#
# Returns:
#   fabric major version as a string
def extract_fabric_version(version):
    version_re = re.compile('(.*)(\d+)\.(\d+)\.(\d+)(.*)')
    match = version_re.match(version)
    if match:
        return '{0}.{1}.{2}'.format(match.group(2), match.group(3), match.group(4))
    else:
        return ''


# Returns average, min/max, etc. statistics of the provided stats file.
# Arguments:
#   config (dict): configuration parameters for the experiment
#   stats_file (string): path to file containing the statistics
#   data (list): list of statistics where each stats is a dictionary
#   columns (dict): mapping of the columns in the stats_file or data to user-defined columns. 
#       If user-defined column name is empty, then uses the same name as defined in the stats_file.
#   skip_lines (int): number of initial lines (after the header) to skip from stats_file
# 
# Returns:
#   processed stats as a dictionary
def process_stats(config, stats_file=None, data=None, columns=None, skip_lines=0):
    # Fill in the missing user-defined column names with the same names from the stats_file.
    for k, v in columns.iteritems():
        if not v:
            columns[k] = k

    stats = {}
    for k, v in columns.iteritems():
        # Columns start and end are to be treated as string.
        if k == 'start' or k == 'end':
            stats[v] = ''
        else:
            stats[v] = 0.0  # Use the user-defined column name.

    ignored_lines = 0
    if stats_file:
        with open(stats_file, 'r') as sf:
            reader = csv.DictReader(sf)
            i = 0
            for line in reader:
                i += 1
                if i <= skip_lines:  # Skip the initial lines
                    continue

                # There are a few outliers for statedb write operations, so this is a hack to
                # ignore those values.
                # if 'statedb_write' in line:
                #     db = config['<db>']
                #     val1 = float(line['statedb_write'])
                #     if (db == 'leveldb' and (val1 > 100.0)) or (db == 'couchdb' and (val1 > 500.0)):
                #         ignored_lines += 1
                #         continue
                    
                for k, v in columns.iteritems():
                    if k == 'start':
                        # Only use the value from the first line after the skipped lines.
                        if i == skip_lines + ignored_lines + 1:
                            stats[v] = line[k]
                    elif k == 'end':
                        stats[v] = line[k]
                    else:
                        stats[v] += float(line[k])  # Let's consider all the other values as floating-point numbers.
    else:
        # Let's process data (which is a list of stats) when stats_file is empty.
        # For now, we assume that data will not have 'start' and 'end' columns.
        i = 0
        for d in data:
            i += 1
            for k, v in columns.iteritems():
                stats[v] += float(d[k])  # Let's consider all the values as floating-point numbers.

    items = i - skip_lines - ignored_lines  # Actual number of items processed.
    if items <= 0:
        return None
    
    for k, v in stats.iteritems():
        if k != 'start' and k != 'end':
            stats[k] = round(v / items, 3)
    stats['items'] = items
    return stats


# Returns name of the peer from the provided file.
# Arguments:
#   file_path (string): path to peer file
#   file_type (string): type of the file, e.g. log file or stats file
#
# Returns:
#   peer name as a string
def get_peer_name(file_path, file_type):
    path_fields = file_path.split('/')

    if file_type == 'log':
        return path_fields[-1].replace('.log', '')
    elif file_type == 'stats':
        return path_fields[-1].replace('committer_stats_', '').replace('.txt', '')
    else:
        print('ERROR: unknown file type for get_peer_name()')


# Prints statistics of all the committers (peers) in a readable format.
# Any special printing is handled here.
#
# Arguments:
#   stats (dict): statistics of all the peers
#
# Returns:
#   None
def print_committer_stats(stats):
    print('\nINFO: *** Peer Statistics ***\n')
    for p, s in sorted(stats.items()):
        print('INFO: {0} -- transactions (succeeded/total) = {1}/{2}'.format(
            p, s['succeeded_measured'], s['transactions']))
        # print('INFO: {0} -- overall latency (s) = {1} overall throughput (tps) = {2}'.format(
        #     p, s['peer_commit'], s['peer_throughput']))
        print('INFO:      with ledger_write -- commit latency (ms) = {0} commit throughput (tps) = {1}'.format(
            s['block_commit'], s['commit_throughput']))
        print('INFO:   without ledger_write -- commit latency (ms) = {0} commit throughput (tps) = {1}'.format(
            s['block_commit_wo_ledger_write'], s['commit_throughput_wo_ledger_write']))
        if 'hw_commit_throughput' in s:
            print('INFO:               hardware -- commit latency (ms) = {0} commit throughput (tps) = {1}'.format(
                s['hw_block_commit'], s['hw_commit_throughput']))
        print('')


# Returns statistics of the provided committer (peer) stats file.
# Arguments:
#   setup (dict): setup for the experiments
#   config (dict): configuration parameters for the experiment
#   stats_file (string): path to file containing the statistics
#   columns (dict): mapping of the columns in the stats_file to user-defined columns
# 
# Returns:
#   stats as a dictionary
def get_committer_stats(setup, config, stats_file, columns):
    # Let's go through all the committer files (from various peers).
    stats = {}
    for f in glob.glob(stats_file):
        s = process_stats(config, stats_file=f, columns=columns, skip_lines=3)
        if not s:
            print('WARNING: stats are empty from {0} !!!'.format(f))
            continue

        stats[get_peer_name(f, 'stats')] = s

    # Updates hardware peer's ledger_write latency from the software peer.
    if setup['hw_peer_type'] == 'non-fabric':
        hp = setup['hw_peer_hp']
        sp = setup['hw_peer_sp']
        if hp in stats.keys() and sp in stats.keys():
            stats[hp]['block_commit'] -= stats[hp]['ledger_write']  # Subtract any existing value.
            stats[hp]['ledger_write'] = stats[sp]['ledger_write']
            stats[hp]['block_commit'] += stats[hp]['ledger_write']

    for p, s in stats.items():
        # Let's compute some more committer stats (latencies are in msecs).
        s['transactions'] = int(s['blocksize_measured'] * s['items'])
        s['succeeded_measured'] = int(s['succeeded_measured'] * s['items'])
        s['commit_throughput'] = int(s['blocksize_measured'] / s['block_commit'] * 1000)
        s['peer_commit'] = (datetime.strptime(s['end'], '%Y-%m-%d %H:%M:%S.%f') -
                           datetime.strptime(s['start'], '%Y-%m-%d %H:%M:%S.%f')).total_seconds()
        s['peer_throughput'] = int(s['transactions'] / s['peer_commit'])
        s['block_commit_wo_ledger_write'] = s['block_commit'] - s['ledger_write']
        s['commit_throughput_wo_ledger_write'] = int(s['blocksize_measured'] / s['block_commit_wo_ledger_write'] * 1000)
        if s['hw_block_commit'] != 0.0:
            s['hw_commit_throughput'] = int(s['blocksize_measured'] / s['hw_block_commit'] * 1000)
        
        if setup['verbose']:
            print('INFO: {0} detailed statistics ...'.format(p))
            print(yaml.dump(s))

    return stats


# Combines various types of peer stats for the provided experiment configuration.
# Arguments:
#   setup (dict): setup for the experiments
#   config (dict): configuration parameters for the experiment
#   columns (dict): mapping of the columns in the stats to user-defined columns
#   combined_stats (list): updates this list with the combined stats
# 
# Returns:
#   None
def combine_stats(setup, config, columns, combined_stats, per_peer=False):
    # Gather stats from all the runs of an experiment configuration.
    stats = []
    committer_stats_file = os.path.join(setup['stats_dir'], 'committer_stats_*.txt')
    stats_committer = get_committer_stats(setup, config, committer_stats_file, columns['committer'])
    print_committer_stats(stats_committer)
    stats.append(stats_committer)

    combined_stats_peer = []
    s = {}

    # Pre-fill the values for key columns which are constant and hence their average is the same.
    # For value columns, the averaged values are computed later and then updated.
    for col in columns['combined_stats_keys']:
        if col != 'peer':
            s[col] = config['<' + col + '>']
    cols = {}
    for col in columns['combined_stats_values']:
        cols[col] = ''

    for peer in stats[0].keys():
        # For each peer, gather its stats from all the runs and compute the average.
        # Assumes that the peers in stats[0].keys() are the same as other stats[*].keys().
        stats_peer = []
        for d in stats:
            stats_peer.append(d[peer])
        s.update({'peer' : peer})
        s.update(process_stats(config, data=stats_peer, columns=cols))
        combined_stats_peer.append(s.copy())

    if per_peer:
        combined_stats.extend(combined_stats_peer)
    else:
        # Let's use the peer which had the best throughput as the overall throughput (which should
        # be oblivious to VM resource issues).
        overall_stats = {'peer_throughput': 0.0}
        for s in combined_stats_peer:
            if s['peer_throughput'] > overall_stats['peer_throughput']:
                overall_stats = s
        overall_stats.update({'peer': 'overall'})
        combined_stats.append(overall_stats)


# Writes the combined stats to the provided output file.
# Arguments:
#   columns (dict): user-defined columns to use for output_file
#   stats (dict): combined statistics
#   output_file (string): path to output file
# 
# Returns:
#   None
def write_combined_stats(columns, stats, output_file):
    # Returns a key for sorting the combined stats.
    # Arguments:
    #   stats (dict): combined statistics
    # 
    # Returns:
    #   None
    def get_combined_stats_key(stats):
        values = []
        for c in columns['combined_stats_keys']:
            if c != 'peer':
                values.append(c)
                values.append(stats[c])
        values.append('peer')
        values.append(stats['peer'])
        return get_string(values, 4)
    stats.sort(key=get_combined_stats_key)

    cols = columns['combined_stats_keys'] + columns['combined_stats_values']
    with open(output_file, 'w') as of:
        writer = csv.DictWriter(of, fieldnames=cols, extrasaction='ignore')
        writer.writeheader()
        for s in stats:
            writer.writerow(s)


# Runs various commands according to the provided setup and parameters.
# Arguments:
#   setup (dict): common setup configuration for all the commands  
#   params (dict): parameters to generate different experiment configurations
#   columns (dict): mapping of the columns in the collected stats to user-defined columns
# 
# Returns:
#   None
def run(setup, params, columns):
    mode = setup['mode']

    config = {}
    combined_stats = []
    if mode == 'combinestats' or mode == 'combinestats-perpeer':
        per_peer = True if mode == 'combinestats-perpeer' else False
        combine_stats(setup, config, columns, combined_stats, per_peer=per_peer)
        write_combined_stats(columns, combined_stats, os.path.join(setup['stats_dir'], 'combined_stats.txt'))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Process statistics from peers and/or orderer.')
    parser.add_argument('-v', dest='verbose', action='store_true', help='Enable verbose output.')
    parser.add_argument('-m', type=str, dest='mode', action='store', required=True,
                        choices = ['combinestats', 'combinestats-perpeer'],
                        help='Mode for this program.')
    parser.add_argument('--stats-dir', type=str, default='.', dest='stats_dir', action='store', help='Directory containing peer stats files.')
    args = parser.parse_args()

    # Common setup.
    setup = {
        # Common parameters.
        'verbose': args.verbose,
        'mode': args.mode,
        'stats_dir': args.stats_dir,

        # Hardware peer parameters.
        # Possible values for hw_peer_type:
        #   non-fabric = hardware peer is run in standalone mode, without Fabric peer integration.
        #   fabric = hardware peer is run as Fabric peer docker.
        'hw_peer_type': 'non-fabric',
        'hw_peer_hp': 'peer2.org1.example.com',
        'hw_peer_sp': 'peer1.org1.example.com',  # Used for comparison.

    }

    # Various parameters (useful when multiple experiments are run).
    params = {}

    # Various columns used in collecting and generating stats.
    columns = {
        'committer': {
            'start': '',
            'end': '',
            'txs': 'blocksize_measured',
            'succeeded': 'succeeded_measured',
            'vscc_blk': 'vscc',
            'statedb_read': '',
            'mvcc_blk': 'mvcc',
            'ledger_write': '',
            'statedb_write': '',
            'oths_blk': 'others',
            'total_blk': 'block_commit',
            'hw_total_blk': 'hw_block_commit',
        },
        # Initial columns used when generating combined statistics.
        'combined_stats_keys': [
            'peer',
        ],
        'combined_stats_values': [
            'blocksize_measured', 'transactions', 'succeeded_measured',
            'vscc', 'statedb_read', 'mvcc', 'ledger_write', 'statedb_write', 'others',
            'block_commit', 'block_commit_wo_ledger_write', 'hw_block_commit', 'peer_commit',
            'commit_throughput', 'commit_throughput_wo_ledger_write', 'peer_throughput',
        ],
    }

    run(setup, params, columns)
