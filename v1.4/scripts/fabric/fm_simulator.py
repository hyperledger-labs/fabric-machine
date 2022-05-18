#!/usr/bin/env python3
#
# Copyright Xilinx Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
#
# This program implements an adhoc simulator for Blockchain/Fabric Machine.
#
# Run this simulator as:
#   ./fm_simulator.py --blocks=10 --block-size=100
#
# To see all the configurable options, use -h option.

import argparse
from datetime import datetime
from collections import deque
from concurrent import futures
import copy
import random
import re
import time

# Regular expressions to extract various types of data.
block_info_re = re.compile(
    '(.*) UTC(.*)Committed block \[(\d+)\] with (\d+) transaction\(s\) in (\d+)us ' +
    '\(state_validation=(\d+)us block_and_pvtdata_commit=(\d+)us state_commit=(\d+)us\)')
# block_info_re = re.compile(
#     '(.*) UTC(.*)Committed block \[(\d+)\] with (\d+) transaction\(s\) in (\d+)ms ' +
#     '\(state_validation=(\d+)ms block_and_pvtdata_commit=(\d+)ms state_commit=(\d+)ms\)')


def extract_block_info(line):
    match = block_info_re.match(line)
    if match:
        return {'block': match.group(3), 'txs': match.group(4)}


def print_config(config):
    version = config['version']
    if version == 1:
        print(("\n=== Fabric Machine v1 ===\n"
               "ecdsa_engine_latency: {0}ms\n"
               "vscc_threads: {1}\n"
               "tx_vscc_ecdsa_engines: {2}\n"
               "blocks: {3}\n"
               "block_size: {4}\n"
               "endorsements: pop={5} prob={6}\n").format(
                    config['ecdsa_engine_latency'],
                    config['vscc_threads'],
                    config['tx_vscc_ecdsa_engines'],
                    config['blocks'],
                    config['block_size'],
                    config['ends_pop'], config['ends_prob']))


# Verifies the given number of endorsements of a transaction using ecdsa engines.
# The verification itself is not real; the function waits for the time it takes for the verification
# and then returns.
#
# Arguments:
#   config (dict): various configuration options
#   tx_num (int): transaction to which the endorsements belong
#   num_ends (int): number of endorsements to verify
#
# Returns:
#   transaction and its endorsements that are verified
def verify_ends(config, tx_num, num_ends):
    start_time = time.perf_counter()

    # For v1, ecdsa engines per tx_vscc instance are used.
    ee = config['tx_vscc_ecdsa_engines'] if config['version'] == 1 else 1
    iters = (num_ends + ee - 1) // ee
    time.sleep(iters * config['ecdsa_engine_latency']/1000)  # convert latency to secs.

    end_time = time.perf_counter()
    if config['verbose']:
        print('tx{0} with {1} ends {2:0.4f}ms'.format(tx_num, num_ends, (end_time - start_time) * 1000))
    return tx_num, num_ends


# Issues endorsements of a transaction for verification.
#
# Arguments:
#   config (dict): various configuration options
#   threads (ThreadPoolExecutor): threads that can execute verify_ends() function
#   fs (Futures): thread events
#   tx (dict): transaction data
#
# Returns:
#   True if endorsements were issued, otherwise False
def issue_ends(config, threads, fs, tx):
    tx_num = tx['tx_num']
    tx_ends = tx['num_ends']
    ends_issued = tx['ends_issued']
    if ends_issued < tx_ends:
        # For v1, all endorsements of a transaction are issued.
        if config['version'] == 1:
            num_ends = tx_ends - ends_issued
            ends_str = '{0}-{1}'.format(ends_issued, tx_ends-1)

        # Issue endorsement(s) for verification using a free thread.
        fs.add(threads.submit(verify_ends, config, tx_num, num_ends))
        tx['ends_issued'] += num_ends
        if config['verbose']:
            print('scheduled tx{0} ends{1}'.format(tx_num, ends_str))
        return True
    else:
        return False


# Issues a transaction.
#
# Arguments:
#   config (dict): various configuration options
#   threads (ThreadPoolExecutor): threads that can execute verify_ends() function
#   fs (Futures): thread events
#   txs (dict): in-progress transactions
#   tx_fifo (queue): transactions not yet issued
#   tx_num (int): transaction to issue
#
# Returns:
#   None
def issue_tx(config, threads, fs, txs, tx_fifo, tx_num):
    tx_ends = tx_fifo.popleft()
    txs[tx_num] = {'tx_num': tx_num, 'num_ends': tx_ends, 'ends_issued': 0, 'ends_finished': 0}

    # If endorsements of a transaction cannot be issued because there aren't any left, then mark
    # the transaction as finished.
    if not issue_ends(config, threads, fs, txs[tx_num]):
        finish_tx(config, txs, tx_num)


# Finishes a transaction (either in-order or out-of-order).
#
# Arguments:
#   config (dict): various configuration options
#   txs (dict): in-progress transactions
#   tx_num (int): transaction to finish
#
# Returns:
#   None
def finish_tx(config, txs, tx_num):
    # We mark the transaction as finished and delete it only if it is the first one in the 
    # in-progress transactions. Otherwise, the finished transaction will be deleted when it moves
    # to the first position.
    txs_finished = []
    for tx_num in txs:
        tx = txs[tx_num]
        if tx['ends_finished'] != tx['num_ends']:
            break
        txs_finished.append(tx_num)
    for tx_num in txs_finished:
        del txs[tx_num]
        if config['verbose']:
            print('finished tx{0}'.format(tx_num))


# Implements the tx_vscc stage with v1 architecture.
#
# Arguments:
#   config (dict): various configuration options
#   threads (ThreadPoolExecutor): threads that can execute verify_ends() function
#   tx_fifo (queue): transactions not yet issued
#   block_num (int): block to validate
#   num_txs (int): number of transactions in the block
#
# Returns:
#   Time spent in msecs
def tx_vscc_v1(config, threads, tx_fifo, block_num, num_txs):
    start_time = time.perf_counter()

    vscc_threads = config['vscc_threads']
    fs = set()
    txs = {}  # In-progress transactions.
    txs_issued = 0
    issued_ends = True  # Whether an endorsement was issued in the previous iteration of the loop.

    # This loop is executed as long as there are transactions to issue or there are transactions
    # in-progress (so we wait for their completion).
    while txs_issued < num_txs or len(txs) != 0:
        if config['verbose']:
            print('\nnum_txs={0} txs_issued={1} txs_progress={2} vscc_threads={3}'.format(
                num_txs, txs_issued, len(txs), len(fs)))
            print(txs)

        # If all threads are busy or no endorsements were issued in the last iteration (because
        # there were no more endorsements to issue), then we need to wait for currently active
        # threads to finish (and mark the corresponding transactions as finished).
        if len(fs) == vscc_threads or not issued_ends:
            ret = futures.wait(fs, return_when=futures.FIRST_COMPLETED)
            for r in ret.done:
                tx_num, num_ends = r.result()
                txs[tx_num]['ends_finished'] += num_ends
                finish_tx(config, txs, tx_num)
            fs = ret.not_done
        issued_ends = False

        # Issue new transactions if there are available threads.
        if len(txs) < vscc_threads and txs_issued < num_txs:
            tx_num = txs_issued
            issue_tx(config, threads, fs, txs, tx_fifo, tx_num)
            txs_issued += 1
            issued_ends = True

    elapsed_time = (time.perf_counter() - start_time) * 1000
    #print('block{0} {1:0.4f}ms'.format(block_num, elapsed_time))
    return elapsed_time


# Simulates Fabric Machine.
#
# Arguments:
#   config (dict): various configuration options
#   block_fifo (queue): blocks
#   tx_fifo (queue): transactions of all the blocks
#
# Returns:
#   Time spent in msecs
def run_simulation(config, block_fifo, tx_fifo):
    peer_log, pl = config['peer_log'], None
    if peer_log:
        pl = open(peer_log, 'w')
        pl.write('Fabric Machine Simulator v1\n\n')

    with futures.ThreadPoolExecutor(max_workers=config['vscc_threads']) as vscc_threads:
        config['version'] = 1
        print_config(config)

        blocks = config['blocks']
        ecdsa_engine_latency = config['ecdsa_engine_latency']
        mvcc_commit_latency = config['mvcc_commit_latency']
        total_txs = 0
        total_latency = 0
        for b in range(blocks):
            block_txs = block_fifo.popleft()
            total_txs += block_txs

            # For block_processor module, we do not simulate block_verify, tx_verify and 
            # tx_mvcc_commit stages here; instead, we use the fact that in a pipelined architecture,
            # latencies of these stages will only contribute once when the pipeline is being filled.
            # Afterwards, tx_vscc stage is the critical stage and contributes the most to block
            # validation latency.
            # TODO (harisj): add protocol_processor latency.
            vscc_latency = tx_vscc_v1(config, vscc_threads, tx_fifo, b, block_txs)
            block_latency = 2*ecdsa_engine_latency + vscc_latency + mvcc_commit_latency
            total_latency += block_latency

            write_block_info(pl, {'block': b, 'txs': block_txs, 'latency': int(round(block_latency))})
            print('block{0} {1:0.1f} + {2:0.1f} + {3:0.1f} + {4:0.1f} = {5:0.1f}ms'.format(
                b, ecdsa_engine_latency, ecdsa_engine_latency, vscc_latency, mvcc_commit_latency, block_latency))

        print('average block latency: {0:0.1f}ms'.format(total_latency/blocks))
        print('average throughput: {0:0.3f}tps'.format(total_txs/(total_latency/1000)))

    if pl:
        pl.close()


# Writes block information in peer's log format.
#
# Arguments:
#   peer_log (File): file object to write to.
#   block_stats (dict): block stats to write.
#
# Returns:
#   None
def write_block_info(peer_log, block_stats):
    if not peer_log:
        return

    block_num = block_stats['block']
    block_txs = block_stats['txs']

    peer_log.write('.....{0} ... START Block Validation for block [{1}]\n'.format(
        datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S.%f UTC'), block_num))
    peer_log.write('.....{0} ... Validated block [{1}] in 0us\n'.format(
        datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S.%f UTC'), block_num))
    peer_log.write('.....{0} ... Block [{1}] transaction validation flags: '.format(
        datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S.%f UTC'), block_num))

    words = block_txs // 32
    first_word = block_txs - (32 * words)
    peer_log.write('0X{:08X} '.format(2**first_word - 1))  # All transactions marked as valid.
    for i in range(words):
        peer_log.write('0XFFFFFFFF ')

    peer_log.write(('\n.....{0} ... Committed block [{1}] with {2} transaction(s) in {3}us '
        '(state_validation=0us block_and_pvtdata_commit=0us state_commit=0us)\n\n').format(
        datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S.%f UTC'), block_num, block_stats['txs'], block_stats['latency']))


# Generates input for simulation based on the provided parameters.
#
# Arguments:
#   config (dict): various configuration options
#   block_fifo (queue): blocks
#   tx_fifo (queue): transactions of all the blocks
#
# Returns:
#   None
def generate_input(config, block_fifo, tx_fifo):
    blocks_file = config['blocks_file']
    if blocks_file:
        with open(blocks_file, 'r') as bf:
            blocks = 0
            blocks_size = 0

            for line in bf:
                line = line.strip()

                block_info = extract_block_info(line)
                if block_info:
                    bs = int(block_info['txs'])
                    blocks += 1
                    blocks_size += bs
                    block_fifo.append(bs)

                    # For each transaction of a block, the number of endorsements are picked based on the
                    # provided probability distribution for endorsements.
                    # TODO(harisj): use endorsements from the input file when present.
                    for t in random.choices(config['ends_pop'], weights=config['ends_prob'], k=bs):
                        tx_fifo.append(t)

            # Overwrite config data.
            config['blocks'] = blocks
            config['block_size'] = blocks_size / blocks

    else:
        for b in range(config['blocks']):
            bs = config['block_size']
            block_fifo.append(bs)

            # For each transaction of a block, the number of endorsements are picked based on the
            # provided probability distribution for endorsements.
            for t in random.choices(config['ends_pop'], weights=config['ends_prob'], k=bs):
                tx_fifo.append(t)

    if config['verbose']:
        print("\n=== Number of endorsements of all transactions ===")
        print(tx_fifo)


# Applies basic validity checks on probability distribution for endorsements.
#
# Arguments:
#   ends_pop (list): possible number of endorsements for each transaction
#   ends_prob (list): probability of selecting a particular number of endorsements
#
# Returns:
#   None
def check_ends_params(ends_pop, ends_prob):
    if len(ends_pop) != len(ends_prob):
        return False
    if sum(ends_prob) != 1.0 or any(i < 0.0 for i in ends_prob):
        return False
    return True


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Adhoc simulator to estimate block validation latency in Blockchain/Fabric Machine.')
    parser.add_argument('-v', dest='verbose', action='store_true', help='Enable verbose output.')
    parser.add_argument('--ecdsa-engine-latency', type=int, default=360, dest='ecdsa_engine_latency', action='store', help='Latency of an ecdsa engine in msecs (default: 360 ms).')
    parser.add_argument('--tx-vscc-ecdsa-engines', type=int, default=2, dest='tx_vscc_ecdsa_engines', action='store', help='Number of ecdsa engines per transaction for vscc operation. Used with --vscc-threads.')
    parser.add_argument('--vscc-threads', type=int, default=8, dest='vscc_threads', action='store', help='Number of transactions to process in parallel during vscc operation.')
    parser.add_argument('--blocks-file', type=str, default=None, dest='blocks_file', action='store', help='File containing blocks data in peer\'s log format for simulation. Overrides --blocks and --blocks-size.')
    parser.add_argument('--blocks', type=int, default=5, dest='blocks', action='store', help='Number of blocks used in simulation.')
    parser.add_argument('--block-size', type=int, default=100, dest='block_size', action='store', help='Number of transactions per block.')
    parser.add_argument('--ends', type=str, default='2:1.0', dest='ends', action='store', help='Number of endorsements per transaction. Format is "e1,e2,e3:p1,p2,p3" where e1 endorsements are generated with p1 probability in a block. When --ends-params is specified, then the format is "e" which is internally converted to endorsements [e//2, e, e+1, (e+1)*2].')
    parser.add_argument('--ends-params', type=str, default='', dest='ends_params', action='store', help='Parameters for generating endorsements per transaction. Format is "alpha,beta" which is internally converted to endorsement probabilities [beta/2, alpha, (1-beta-alpha), beta/2]. Used with --ends.')
    parser.add_argument('--peer-log', type=str, default=None, dest='peer_log', action='store', help='File for writing simulation output in peer\'s log format.')
    args = parser.parse_args()

    if args.ends_params:
        ends = int(args.ends)
        params = args.ends_params.split(',')
        alpha = float(params[0])
        beta = float(params[1])
        ends_pop = [ends//2, ends, (ends + 1), (ends + 1)*2]
        ends_prob = [beta/2, alpha, (1 - beta - alpha), beta/2]
    else:
        ends_params = args.ends.split(':')
        ends_pop = [int(i) for i in ends_params[0].split(',')]
        ends_prob = [float(i) for i in ends_params[1].split(',')]
    if not check_ends_params(ends_pop, ends_prob):
        print('ERROR: Endorsement parameters are incorrect: pop={0} prob={1}'.format(ends_pop, ends_prob))
        exit()

    config = {
        'verbose': args.verbose,
        'ecdsa_engine_latency': args.ecdsa_engine_latency,
        'mvcc_commit_latency': 10,  # msecs.
        'tx_vscc_ecdsa_engines': args.tx_vscc_ecdsa_engines,
        'vscc_threads': args.vscc_threads,
        'blocks_file': args.blocks_file,
        'blocks': args.blocks,
        'block_size': args.block_size,
        'ends_pop': ends_pop,
        'ends_prob': ends_prob,
        'peer_log': args.peer_log
    }
    block_fifo = deque()
    tx_fifo = deque()

    generate_input(config, block_fifo, tx_fifo)
    run_simulation(config, block_fifo, tx_fifo)
