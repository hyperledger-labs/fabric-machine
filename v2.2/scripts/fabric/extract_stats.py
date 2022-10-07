#!/usr/bin/env python
#
# Copyright Xilinx Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
#
# This program reads peer log files to gather stats related to blocks and transactions.
#
# Run this utility as:
#   ./extract_stats.py --log-files 'peer*.example.com.log'

import argparse
import glob
import logging
import os
import re
import yaml

# Global variables.
verbose = False

# Headers for stats of various phases (e.g. endorse, etc.)
headers = {
    'commit': [
        'blk', 'txs', 'succeeded', 'start', 'end',
        'vscc_blk',
        'statedb_read', 'mvcc_blk',
        'ledger_write', 'statedb_write',
        'commit_blk', 'commit_misc',
        'hw_total_blk',
        'total_blk', 'total_blk_wo_ledger_write',
    ]
}

# Regular expressions to extract various stats.
block_commit_start_re = re.compile('(.*) UTC(.*)Received block \[(\d+)\] from buffer')
block_vscc_re = re.compile('(.*)Validated block \[(\d+)\] in (\d+)us')
block_statedb_read_re = re.compile('(.*)Bulk read of block (\d+) took (\d+)us')
block_mvcc_re = re.compile('(.*)Finished block \[(\d+)\] state validation in (\d+)us')
block_txs_vld_flags_re = re.compile(
    '(.*) UTC(.*)Block \[(\d+)\] transaction validation flags: ([a-zA-Z0-9 ]+)$')
block_hw_commit_re = re.compile(
    '(.*)Hardware committed block \[(\d+)\] with (\d+) transaction\(s\) in (\d+)us')
block_serial_commit_re = re.compile(
    '(.*) UTC(.*)Committed block \[(\d+)\] with (\d+) transaction\(s\) in (\d+)us ' +
    '\(state_validation=(\d+)us block_and_pvtdata_commit=(\d+)us state_commit=(\d+)us\)')
block_parallel_commit_re = re.compile(
    '(.*) UTC(.*)Committed block \[(\d+)\] with (\d+) transaction\(s\) asynchronously in (\d+)us ' +
    '\(state_validation=(\d+)us\)')
block_ledger_write_re = re.compile('(.*) UTC(.*)Committed pvtdata and block \[(\d+)\] to ledger in (\d+)us')
block_statedb_write_re = re.compile('(.*) UTC(.*)Committed block \[(\d+)\] to state database in (\d+)us')


def extract_block_commit_start_info(line):
    match = block_commit_start_re.match(line)
    if match:
        return {'blk': int(match.group(3)), 'start': match.group(1)[5:]}


def extract_block_vscc_info(line):
    match = block_vscc_re.match(line)
    if match:
        return {'blk': int(match.group(2)), 'vscc_blk': int(match.group(3))}


def extract_block_statedb_read_info(line):
    match = block_statedb_read_re.match(line)
    if match:
        return {'blk': int(match.group(2)), 'statedb_read': int(match.group(3))}


def extract_block_txs_vld_info(line):
    match = block_txs_vld_flags_re.match(line)
    if match:
        valid_txs = 0
        for w in match.group(4).split(' '):
            # Convert each word representing validation flags (0xAFFFFFFB) to its integer equivalent,
            # and then count the number of 1s in its binary format.
            valid_txs += bin(int(w, 16)).count('1')
        return {'blk': int(match.group(3)), 'txs_vld_flags': match.group(4), 'succeeded': int(valid_txs)} 


def extract_block_hw_commit_info(line):
    match = block_hw_commit_re.match(line)
    if match:
        return {'blk': int(match.group(2)), 'hw_total_blk': int(match.group(4))}


def extract_block_commit_info(line):
    match = block_serial_commit_re.match(line)
    if match:
        return {'commit_type': 'serial', 'ledger_write_done': True, 'statedb_write_done': True, 'commit_done': True,
            'blk': int(match.group(3)), 'end': match.group(1)[5:], 'txs': int(match.group(4)),
            'mvcc_blk': int(match.group(6)), 'ledger_write': int(match.group(7)),
            'statedb_write': int(match.group(8)), 'commit_blk': int(match.group(5))}

    match = block_parallel_commit_re.match(line)
    if match:
        return {'commit_type': 'parallel', 'commit_done': True,
            'blk': int(match.group(3)), 'end': match.group(1)[5:], 'txs': int(match.group(4)),
            'mvcc_blk': int(match.group(6)), 'commit_blk': int(match.group(5))}

 
def extract_block_ledger_write_info(line):
    match = block_ledger_write_re.match(line)
    if match:
        return {'ledger_write_done': True,
            'blk': int(match.group(3)), 'end': match.group(1)[5:], 'ledger_write': int(match.group(4))}


def extract_block_statedb_write_info(line):
    match = block_statedb_write_re.match(line)
    if match:
        return {'statedb_write_done': True,
            'blk': int(match.group(3)), 'end': match.group(1)[5:], 'statedb_write': int(match.group(4))}


# Extracts and updates commit stats of a block.
# Arguments:
#   line: a string containing the text
#   peer_stats: a dictionary containing stats of multiple blocks
#
# Returns:
#   True if stats were extracted and updated, False otherwise.
def extract_update_commit_stats(line, peer_stats):
    stats = extract_block_vscc_info(line)
    if stats:
        update_block_stats(peer_stats, stats)
        return True

    stats = extract_block_statedb_read_info(line)
    if stats:
        update_block_stats(peer_stats, stats)
        return True

    stats = extract_block_hw_commit_info(line)
    if stats:
        update_block_stats(peer_stats, stats)
        return True

    stats = extract_block_txs_vld_info(line)
    if stats:
        update_block_stats(peer_stats, stats)
        return True

    stats = extract_block_commit_info(line)
    if stats:
        update_block_stats(peer_stats, stats)
        return True

    stats = extract_block_statedb_write_info(line)
    if stats:
        update_block_stats(peer_stats, stats)
        return True

    stats = extract_block_ledger_write_info(line)
    if stats:
        update_block_stats(peer_stats, stats)
        return True

    return False


# Initializes stats of a block.
# Arguments:
#   stats_type: type of stats
#   peer_stats: a dictionary containing stats of multiple blocks
#   block: a dictionary containing block info
#
# Returns:
#   None
def init_block_stats(stats_type, peer_stats, block):
    block_stats = {}
    for h in headers[stats_type]:
        if h == 'start' or h == 'end':
            block_stats[h] = ''
        else:
            block_stats[h] = 0
    block_stats['done'] = False
    block_stats['ledger_write_done'] = False
    block_stats['statedb_write_done'] = False
    block_stats['commit_done'] = False
    block_stats.update(block)
    peer_stats[block['blk']] = block_stats


# Updates existing block stats with the provided stats.
# Arguments:
#   peer_stats: a dictionary containing stats of multiple blocks
#   stats: a dictionary containing stats to be updated
#
# Returns:
#   None
def update_block_stats(peer_stats, stats):
    block_stats = peer_stats[stats['blk']]
    block_stats.update(stats)


# Checks whether all stats of a block have been extracted.
# Arguments:
#   block_stats: a dictionary containing block stats
#
# Returns:
#   True if all block stats are available, False otherwise.
def all_block_stats_extracted(block_stats):
    if block_stats['done']:
        return True
    else:
        block_stats['done'] = block_stats['ledger_write_done'] & block_stats['statedb_write_done'] & block_stats['commit_done']
        return block_stats['done']


# Computes final stats of a block (based on extracted stats).
# Arguments:
#   block_stats: a dictionary containing block stats
#
# Returns:
#   True if computation of final block stats succeeded, False otherwise.
def compute_block_stats(block_stats):
    if not all_block_stats_extracted(block_stats):
        return False

    if block_stats['commit_type'] == 'serial':
        block_stats['mvcc_blk'] -= block_stats['statedb_read']
        block_stats['commit_misc'] = block_stats['commit_blk'] - \
            (block_stats['statedb_read'] + block_stats['mvcc_blk'] + \
            block_stats['ledger_write'] + block_stats['statedb_write'])
        block_stats['total_blk'] = block_stats['vscc_blk'] + block_stats['commit_blk']
        block_stats['total_blk_wo_ledger_write'] = block_stats['total_blk'] - block_stats['ledger_write']
        return True

    if block_stats['commit_type'] == 'parallel':
        block_stats['mvcc_blk'] -= block_stats['statedb_read']
        block_stats['commit_misc'] = block_stats['commit_blk'] - \
            (block_stats['statedb_read'] + block_stats['mvcc_blk'])

        add_ledger_statedb_write = max(block_stats['ledger_write'], block_stats['statedb_write'])
        sub_ledger_write = max(block_stats['ledger_write'] - block_stats['statedb_write'], 0)
        block_stats['total_blk'] = block_stats['vscc_blk'] + block_stats['commit_blk'] + add_ledger_statedb_write
        block_stats['total_blk_wo_ledger_write'] = block_stats['total_blk'] - sub_ledger_write
        return True

    return False


# Extracts start of a block.
# Arguments:
#   line: a string containing the text
#   stats_type: type of stats
#
# Returns:
#   block info as a dictionary or None
def extract_block_start(line, stats_type):
    if stats_type == 'commit':
        return extract_block_commit_start_info(line)
    else:
        print('ERROR: Unknown stats type {0}'.format(stats_type))


# Extracts and updates stats of a block.
# Arguments:
#   line: a string containing the text
#   stats_type: type of stats
#   peer_stats: a dictionary containing stats of multiple blocks
#
# Returns:
#   True if stats were extracted and updated, False otherwise.
def extract_update_block_stats(line, stats_type, peer_stats):
    if stats_type == 'commit':
        return extract_update_commit_stats(line, peer_stats)
    else:
        print('ERROR: Unknown stats type {0}'.format(stats_type))
        return False


# Removes a block's stats.
# Arguments:
#   peer_stats: a dictionary containing stats of multiple blocks
#   block_num: an int containing block number (whos stats should be removed)
# 
# Returns:
#   None
def remove_block_stats(peer_stats, block_num):
    peer_stats.pop(block_num)


# Formats the header line for stats.
# Arguments:
#   stats_type: type of stats
#
# Returns:
#   a string of comma-separated fields
def format_header(stats_type):
    return ','.join(headers[stats_type]) + '\n'


# Transforms the provided stats.
# Arguments:
#   stats_type: type of stats
#   stats: a dictionary containing various block stats
#
# Returns:
#   None
def transform_block_stats(stats_type, block_stats):
    for h in headers[stats_type]:
        if h != 'blk' and h != 'txs' and h != 'succeeded' and h != 'start' and h != 'end':
            block_stats[h] /= 1000.0  # microseconds to milliseconds


# Formats the provided stats as comma-separated stats.
# Arguments:
#   stats_type: type of stats
#   stats: a dictionary containing various block stats
#
# Returns:
#   a string of comma-separated block stats
def format_block_stats(stats_type, block_stats):
    values = []
    for h in headers[stats_type]:
        if h == 'blk' or h == 'txs' or h == 'succeeded' or h == 'start' or h == 'end':
            values.append(str(block_stats[h]))
        else:
            values.append('{0:.3f}'.format(block_stats[h]))
    return ','.join(values) + '\n'


# Extracts stats from peer log file.
# Arguments:
#   log_file: a string containing path to log file
#
# Returns:
#   None
def extract_stats_from_peer(log_file):
    # Let's open stats files and write the headers.
    commit_stats_file = os.path.join(os.path.dirname(log_file),
        'committer_stats_' + os.path.splitext(os.path.basename(log_file))[0] + '.txt')
    stats_files = {
        'commit': open(commit_stats_file, 'w'),
    }
    print('INFO: peer_log_file: {0}'.format(log_file))
    for stats_type, sf in stats_files.iteritems():
        print('INFO: {0}_stats_file: {1}'.format(stats_type, sf.name))
        sf.write(format_header(stats_type))

    # Let's go through the log file.
    peer_stats = {}
    block_to_write = None
    with open(log_file, 'r') as lf:
        for line in lf:
            line = line.strip()

            # This is unnecessary, but will be needed when more phases are added (e.g. endorse, etc.).
            stats_type = 'commit'

            block = extract_block_start(line, stats_type)
            if block:
                # Found a new block, so initialize its stats (block0 is ignored).
                block_num = block['blk']
                if block_num == 0:
                    continue
                init_block_stats(stats_type, peer_stats, block)

                # If it's the very first block, then initialize the variable used for writing stats.
                if block_to_write is None:
                    block_to_write = block_num

                if verbose:
                    print('INFO: {0}'.format(line))
                    print(yaml.dump(peer_stats))

                # Skip the remaining extraction logic since this line is already done.
                continue

            # Keep extracting and writing block stats after we have found the first block.
            if block_to_write is not None:
                if not extract_update_block_stats(line, stats_type, peer_stats):
                    # If nothing could be extracted from this line, then skip the remaining logic.
                    continue

                if verbose:
                    print('INFO: {0}'.format(line))
                    print(yaml.dump(peer_stats))

                # Write block stats when they are done, and release memory.
                block_stats = peer_stats.get(block_to_write)
                if block_stats and compute_block_stats(block_stats):
                    if verbose:
                        print('INFO: Going to write block{0} stats ...'.format(block_to_write))
                        print(yaml.dump(peer_stats))

                    transform_block_stats(stats_type, block_stats)
                    stats_files[stats_type].write(format_block_stats(stats_type, block_stats))
                    remove_block_stats(peer_stats, block_to_write)
                    block_to_write += 1

    if peer_stats:
        print('ERROR: First block with incomplete stats ...')
        print(yaml.dump(peer_stats[block_to_write]))
        print('ERROR: All blocks with incomplete stats ... ')
        for s in peer_stats.values():
            if not s['commit_done']:
                print(yaml.dump(s))
        print('ERROR: Could not extract all stats for the above block(s)!!!')

    # Let's close all stats files.
    for _, sf in stats_files.iteritems():
        sf.close()
    print('')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Extract statistics from peer logs.')
    parser.add_argument('--log-files', type=str, dest='log_files', action='store', help='File(s) containing peer outputs.')
    parser.add_argument('-v', dest='verbose', action='store_true', default=None, help='Enable verbose output.')
    args = parser.parse_args()
    verbose = args.verbose

    if args.log_files:
        # Let's go through the peer log files one by one.
        for f in glob.glob(args.log_files):
            extract_stats_from_peer(f)
