#!/usr/bin/env python
#
# Copyright Xilinx Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
#
# This program reads the log file of the peer to gather stats related to blocks and transactions.
#
# Run this utility as:
#   ./extract_stats.py --log_files 'peer*.example.com.log'

import argparse
import glob
import logging
import os
import re

# Global variables.
# Creates a logger for the provided name.
def get_logger(name):
    handler = logging.StreamHandler()
    handler.setFormatter(logging.Formatter("%(levelname)s %(message)s"))
    logger = logging.getLogger(name)
    logger.addHandler(handler)
    logger.setLevel(logging.INFO)
    return logger
logger = get_logger('stats')

# Different phases of a transaction (e.g. endorse, etc.)
phases = {
    'commit': ['vscc', 'mvcc', 'write'],
}

# Header for stats of a transaction phase (e.g. endorse, etc.)
headers = {
    'commit': ['blk', 'txs', 'succeeded', 'start', 'end',
        'vscc_txs', 'vscc_blk',
        'statedb_read', 'mvcc_txs', 'mvcc_oths', 'mvcc_blk',
        'ledger_write', 'statedb_write',
        'commit_oths', 'commit_blk',
        'oths_txs', 'oths_blk', 'total_txs', 'total_blk',
        'hw_total_blk']
}

# Regular expressions to extract various stats.
block_commit_start_re = re.compile('(.*) UTC(.*)START Block Validation for block \[(\d+)\]')
transaction_vscc_re = re.compile('(.*)VSCC of block (\d+) tx (\d+) took (\d+)us')
block_vscc_re = re.compile('(.*)Validated block \[(\d+)\] in (\d+)us')
# block_vscc_re = re.compile('(.*)Validated block \[(\d+)\] in (\d+)ms')
block_statedb_read_re = re.compile('(.*)Bulk read of block (\d+) took (\d+)us')
transaction_mvcc_re = re.compile('(.*)MVCC of block (\d+) tx (\d+) took (\d+)us')
hw_commit_re = re.compile(
    '(.*)Hardware committed block \[(\d+)\] with (\d+) transaction\(s\) in (\d+)us')
block_txs_vld_flags_re = re.compile(
    '(.*) UTC(.*)Block \[(\d+)\] transaction validation flags: ([a-zA-Z0-9 ]+)$')
block_commit_end_re = re.compile(
    '(.*) UTC(.*)Committed block \[(\d+)\] with (\d+) transaction\(s\) in (\d+)us ' +
    '\(state_validation=(\d+)us block_and_pvtdata_commit=(\d+)us state_commit=(\d+)us\)')
# block_commit_end_re = re.compile(
#     '(.*) UTC(.*)Committed block \[(\d+)\] with (\d+) transaction\(s\) in (\d+)ms ' +
#     '\(state_validation=(\d+)ms block_and_pvtdata_commit=(\d+)ms state_commit=(\d+)ms\)')


# Extracts the details of a block.
# Arguments:
#   line: a string containing the text
#
# Returns:
#   block info as a dictionary
def extract_block_info(stats_type, line):
    if stats_type == 'commit':
        match = block_commit_start_re.match(line)
        if match:
            return {'blk': match.group(3), 'start': match.group(1)[5:]}
    else:
        logger.error('Unknown stats type {0}'.format(stats_type))


def extract_transaction_vscc_info(line):
    match = transaction_vscc_re.match(line)
    if match:
        return {'blk': match.group(2), 'tx': match.group(3), 'vscc_txs': match.group(4)}


def extract_block_vscc_info(line):
    match = block_vscc_re.match(line)
    if match:
        return {'blk': match.group(2), 'vscc_blk': match.group(3)}


def extract_block_statedb_read_info(line):
    match = block_statedb_read_re.match(line)
    if match:
        return {'blk': match.group(2), 'statedb_read': match.group(3)}


def extract_transaction_mvcc_info(line):
    match = transaction_mvcc_re.match(line)
    if match:
        return {'blk': match.group(2), 'tx': match.group(3), 'mvcc_txs': match.group(4)}


def extract_block_txs_vld_info(line):
    match = block_txs_vld_flags_re.match(line)
    if match:
        valid_txs = 0
        for w in match.group(4).split(' '):
            # Convert each word representing validation flags (0xAFFFFFFB) to its integer equivalent,
            # and then count the number of 1s in its binary format.
            valid_txs += bin(int(w, 16)).count('1')
        return {'blk': match.group(3), 'txs_vld_flags': match.group(4), 'succeeded': valid_txs} 


def extract_block_commit_info(line):
    match = block_commit_end_re.match(line)
    if match:
        return {'blk': match.group(3), 'end': match.group(1)[5:], 'txs': match.group(4), 'mvcc_blk': match.group(6),
            'ledger_write': match.group(7), 'statedb_write': match.group(8), 'commit_blk': match.group(5)}


def extract_hw_commit_info(line):
    match = hw_commit_re.match(line)
    if match:
        return {'blk': match.group(2), 'hw_total_blk': match.group(4)}


# Checks whether the block info matches or not.
# Arguments:
#   expected and actual: ints containing block numbers
#
# Returns:
#   true or false
def check_block(expected, actual):
    if expected != actual:
        logger.warning('Expected block={0} but found {1}.'.format(expected, actual))
        return False
    return True


# Initializes the stats for a block based on the type of stats.
# Arguments:
#   stats_type: type of stats
#   stats: a dictionary containing various stats
#   block: int containing block number
#
# Returns:
#   dictionary containing various stats
def init_block_stats(stats_type, stats, block):
    for h in headers[stats_type]:
        if h == 'start' or h == 'end':
            stats[h] = ''
        else:
            stats[h] = 0
    stats['phase'] = 0
    stats.update(block)
    return stats


# Extracts the stats for a block based on the type of stats.
# Arguments:
#   line: a string containing the text
#   stats_type: type of stats
#   stats: a dictionary containing various stats
#
# Returns:
#   None
def extract_block_stats(line, stats_type, stats):
    if stats_type == 'commit':
        extract_commit_stats(line, stats)
    else:
        logger.error('Unknown stats type {0}'.format(stats_type))


# Extracts the commit stats for a block.
# Arguments:
#   line: a string containing the text
#   stats: a dictionary containing various stats
#
# Returns:
#   None
def extract_commit_stats(line, stats):
    if stats['phase'] == 0:  # vscc phase
        result = extract_transaction_vscc_info(line)
        if result and check_block(stats['blk'], result['blk']):
                stats['vscc_txs'] += int(result['vscc_txs'])
        else:
            result = extract_block_vscc_info(line)
            if result and check_block(stats['blk'], result['blk']):
                    stats['vscc_blk'] = int(result['vscc_blk'])
                    stats['phase'] += 1  # done with vscc phase

    elif stats['phase'] == 1:  # mvcc phase
        result = extract_transaction_mvcc_info(line)
        if result and check_block(stats['blk'], result['blk']):
            stats['mvcc_txs'] += int(result['mvcc_txs'])
        else:
            result = extract_block_statedb_read_info(line)
            if result and check_block(stats['blk'], result['blk']):
                stats['statedb_read'] = int(result['statedb_read'])
            else:
                result = extract_hw_commit_info(line)
                if result and check_block(stats['blk'], result['blk']):
                    stats['hw_total_blk'] = int(result['hw_total_blk'])
                else:
                    result = extract_block_txs_vld_info(line)
                    if result and check_block(stats['blk'], result['blk']):
                        stats['succeeded'] = int(result['succeeded'])
                    else:
                        result = extract_block_commit_info(line)
                        if result and check_block(stats['blk'], result['blk']):
                            stats.update({k: int(v) if k != 'end' else v for k, v in result.iteritems()})

                            # Calculate the rest of the stats
                            stats['mvcc_blk'] -= stats['statedb_read']
                            stats['mvcc_oths'] = stats['mvcc_blk'] - stats['mvcc_txs']
                            stats['commit_oths'] = stats['commit_blk'] - \
                                (stats['statedb_read'] + stats['mvcc_blk'] + \
                                stats['ledger_write'] + stats['statedb_write'])
                            stats['oths_txs'] = stats['mvcc_oths'] + stats['commit_oths']
                            stats['oths_blk'] = stats['commit_oths']

                            stats['total_txs'] = stats['vscc_blk'] + stats['statedb_read'] + \
                                stats['mvcc_txs'] + stats['mvcc_oths'] + \
                                stats['ledger_write'] + stats['statedb_write'] + stats['commit_oths']
                            stats['total_blk'] = stats['vscc_blk'] + stats['commit_blk']

                            stats['phase'] += 2  # done with both mvcc and write phases


# Formats the header line based on the type of stats.
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
#   stats: a dictionary containing various stats
#
# Returns:
#   None
def transform_stats(stats_type, stats):
    for h in headers[stats_type]:
        if h != 'blk' and h != 'txs' and h != 'succeeded' and h != 'start' and h != 'end':
            stats[h] /= 1000.0  # microseconds to milliseconds


# Formats the provided stats as comma-separated stats.
# Arguments:
#   stats_type: type of stats
#   stats: a dictionary containing various stats
#
# Returns:
#   a string of comma-separated stats
def format_stats(stats_type, stats):
    values = []
    for h in headers[stats_type]:
        if h == 'blk' or h == 'txs' or h == 'succeeded' or h == 'start' or h == 'end':
            values.append(str(stats[h]))
        else:
            values.append('{0:.3f}'.format(stats[h]))
    return ','.join(values) + '\n'


# Extracts the stats from a peer's log file.
# Arguments:
#   log_file: a string containing path to log file
#
# Returns:
#   None
def extract_stats_from_peer(log_file):
    # Let's open the files and write the headers.
    stats_files = {
        'commit': open(os.path.join(
            os.path.dirname(log_file), 'committer_stats_' + os.path.splitext(os.path.basename(log_file))[0] + '.txt'), 'w'),
    }
    for stats_type, sf in stats_files.iteritems():
        sf.write(format_header(stats_type))

    # Let's go through the log file.
    with open(log_file, 'r') as lf:
        block_stats = {
            'commit': init_block_stats('commit', {}, {}),
        }

        for line in lf:
            line = line.strip()
            
            # TODO: This is unnecessary, but will be needed when more phases are added (e.g. endorse, etc.).
            stats_type = 'commit'
            stats = block_stats[stats_type]

            block = extract_block_info(stats_type, line)
            if block:
                logger.info('block={0}'.format(block))
                if stats['blk']:
                    logger.warning('Found a new block without the complete {0} stats for the previous block.'.format(stats_type))
                # We match the stats with the most recent block, so let's discard
                # the previous block.
                init_block_stats(stats_type, stats, block)
                continue

            if stats['blk']:
                extract_block_stats(line, stats_type, stats)
                # Check whether stats for all the operations have been gathered or not.
                if stats['phase'] == len(phases[stats_type]):
                    transform_stats(stats_type, stats)
                    #logger.info('block_{0}_stats={1}'.format(stats_type, stats))
                    stats_files[stats_type].write(format_stats(stats_type, stats))
                    init_block_stats(stats_type, stats, {})  # reset the stats

    for _, sf in stats_files.iteritems():
        sf.close()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Extract statistics from the peer logs.')
    parser.add_argument('--log-files', type=str, dest='log_files', action='store', help='File(s) containing peer outputs.')
    args = parser.parse_args()

    if args.log_files:
        # Let's go through all the peer files.
        for f in glob.glob(args.log_files):
            extract_stats_from_peer(f)
