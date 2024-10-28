import argparse
import logging
import os
import subprocess
import json
import socket
import datetime
import time
import shutil
import http.client
from multiprocessing import Process, Queue
import concurrent.futures
from collections import namedtuple
# This import is necessary for devnet logs to be shown.
from . import log_setup


pjoin = os.path.join

parser = argparse.ArgumentParser(description='Bedrock devnet launcher')
parser.add_argument('--monorepo-dir', help='Directory of the monorepo', default=os.getcwd())
parser.add_argument('--allocs', help='Only create the allocs and exit', type=bool, action=argparse.BooleanOptionalAction)

log = logging.getLogger()

# Global constants
FORKS = ["delta", "ecotone", "fjord", "granite", "holocene"]

# Global environment variables
DEVNET_NO_BUILD = os.getenv('DEVNET_NO_BUILD') == "true"
DEVNET_L2OO = os.getenv('DEVNET_L2OO') == "true"
DEVNET_ALTDA = os.getenv('DEVNET_ALTDA') == "true"
GENERIC_ALTDA = os.getenv('GENERIC_ALTDA') == "true"

class Bunch:
    def __init__(self, **kwds):
        self.__dict__.update(kwds)

class ChildProcess:
    def __init__(self, func, *args):
        self.errq = Queue()
        self.process = Process(target=self._func, args=(func, args))

    def _func(self, func, args):
        try:
            func(*args)
        except Exception as e:
            self.errq.put(str(e))

    def start(self):
        self.process.start()

    def join(self):
        self.process.join()

    def get_error(self):
        return self.errq.get() if not self.errq.empty() else None


def main():
    args = parser.parse_args()

    monorepo_dir = os.path.abspath(args.monorepo_dir)
    devnet_dir = pjoin(monorepo_dir, '.devnet')
    contracts_bedrock_dir = pjoin(monorepo_dir, 'packages', 'contracts-bedrock')
    deployment_dir = pjoin(contracts_bedrock_dir, 'deployments', 'devnetL1')
    forge_l1_dump_path = pjoin(contracts_bedrock_dir, 'state-dump-900.json')
    op_node_dir = pjoin(args.monorepo_dir, 'op-node')
    ops_bedrock_dir = pjoin(monorepo_dir, 'ops-bedrock')
    deploy_config_dir = pjoin(contracts_bedrock_dir, 'deploy-config')
    devnet_config_path = pjoin(deploy_config_dir, 'devnetL1.json')
    devnet_config_template_path = pjoin(deploy_config_dir, 'devnetL1-template.json')
    ops_chain_ops = pjoin(monorepo_dir, 'op-chain-ops')

    paths = Bunch(
      mono_repo_dir=monorepo_dir,
      devnet_dir=devnet_dir,
      contracts_bedrock_dir=contracts_bedrock_dir,
      deployment_dir=deployment_dir,
      forge_l1_dump_path=forge_l1_dump_path,
      l1_deployments_path=pjoin(deployment_dir, '.deploy'),
      deploy_config_dir=deploy_config_dir,
      devnet_config_path=devnet_config_path,
      devnet_config_template_path=devnet_config_template_path,
      op_node_dir=op_node_dir,
      ops_bedrock_dir=ops_bedrock_dir,
      ops_chain_ops=ops_chain_ops,
      genesis_l1_path=pjoin(devnet_dir, 'genesis-l1.json'),
      genesis_l2_path=pjoin(devnet_dir, 'genesis-l2.json'),
      allocs_l1_path=pjoin(devnet_dir, 'allocs-l1.json'),
      addresses_json_path=pjoin(devnet_dir, 'addresses.json'),
      sdk_addresses_json_path=pjoin(devnet_dir, 'sdk-addresses.json'),
      rollup_config_path=pjoin(devnet_dir, 'rollup.json')
    )

    os.makedirs(devnet_dir, exist_ok=True)

    if args.allocs:
        devnet_l1_allocs(paths)
        devnet_l2_allocs(paths)
        return

    git_commit = subprocess.run(['git', 'rev-parse', 'HEAD'], capture_output=True, text=True).stdout.strip()
    git_date = subprocess.run(['git', 'show', '-s', "--format=%ct"], capture_output=True, text=True).stdout.strip()

    # CI loads the images from workspace, and does not otherwise know the images are good as-is
    if DEVNET_NO_BUILD:
        log.info('Skipping docker images build')
    else:
        log.info(f'Building docker images for git commit {git_commit} ({git_date})')
        run_command(['docker', 'compose', 'build', '--progress', 'plain',
                     '--build-arg', f'GIT_COMMIT={git_commit}', '--build-arg', f'GIT_DATE={git_date}'],
                    cwd=paths.ops_bedrock_dir, env={
            'PWD': paths.ops_bedrock_dir,
            'DOCKER_BUILDKIT': '1', # (should be available by default in later versions, but explicitly enable it anyway)
            'COMPOSE_DOCKER_CLI_BUILD': '1'  # use the docker cache
        })

    log.info('Devnet starting')
    devnet_deploy(paths)

def init_devnet_l1_deploy_config(paths, update_timestamp=False):
    deploy_config = read_json(paths.devnet_config_template_path)
    if update_timestamp:
        deploy_config['l1GenesisBlockTimestamp'] = '{:#x}'.format(int(time.time()))
    if DEVNET_L2OO:
        deploy_config['useFaultProofs'] = False
    if DEVNET_ALTDA:
        deploy_config['useAltDA'] = True
    if GENERIC_ALTDA:
        deploy_config['daCommitmentType'] = "GenericCommitment"
    write_json(paths.devnet_config_path, deploy_config)

def devnet_l1_allocs(paths):
    log.info('Generating L1 genesis allocs')
    init_devnet_l1_deploy_config(paths)

    fqn = 'scripts/deploy/Deploy.s.sol:Deploy'
    run_command([
        # We need to set the sender here to an account we know the private key of,
        # because the sender ends up being the owner of the ProxyAdmin SAFE
        # (which we need to enable the Custom Gas Token feature).
        'forge', 'script', fqn, "--sig", "runWithStateDump()", "--sender", "0x90F79bf6EB2c4f870365E785982E1f101E93b906"
    ], env={
      'DEPLOYMENT_OUTFILE': paths.l1_deployments_path,
      'DEPLOY_CONFIG_PATH': paths.devnet_config_path,
    }, cwd=paths.contracts_bedrock_dir)

    shutil.move(src=paths.forge_l1_dump_path, dst=paths.allocs_l1_path)

    shutil.copy(paths.l1_deployments_path, paths.addresses_json_path)


def devnet_l2_allocs(paths):
    log.info('Generating L2 genesis allocs, with L1 addresses: '+paths.l1_deployments_path)

    fqn = 'scripts/L2Genesis.s.sol:L2Genesis'
    run_command([
        'forge', 'script', fqn, "--sig", "runWithAllUpgrades()"
    ], env={
      'CONTRACT_ADDRESSES_PATH': paths.l1_deployments_path,
      'DEPLOY_CONFIG_PATH': paths.devnet_config_path,
    }, cwd=paths.contracts_bedrock_dir)

    # For the previous forks, and the latest fork (default, thus empty prefix),
    # move the forge-dumps into place as .devnet allocs.
    for fork in FORKS:
        input_path = pjoin(paths.contracts_bedrock_dir, f"state-dump-901-{fork}.json")
        output_path = pjoin(paths.devnet_dir, f'allocs-l2-{fork}.json')
        shutil.move(src=input_path, dst=output_path)
        log.info("Generated L2 allocs: "+output_path)


# Bring up the devnet where the contracts are deployed to L1
def devnet_deploy(paths):
    if os.path.exists(paths.genesis_l1_path):
        log.info('L1 genesis already generated.')
    else:
        log.info('Generating L1 genesis.')
        if not os.path.exists(paths.allocs_l1_path) or DEVNET_L2OO or DEVNET_ALTDA:
            # If this is a devnet variant then we need to generate the allocs
            # file here always. This is because CI will run devnet-allocs
            # without setting the appropriate env var which means the allocs will be wrong.
            # Re-running this step means the allocs will be correct.
            devnet_l1_allocs(paths)
        else:
            log.info('Re-using existing L1 allocs.')

        # It's odd that we want to regenerate the devnetL1.json file with
        # an updated timestamp different than the one used in the devnet_l1_allocs
        # function.  But, without it, CI flakes on this test rather consistently.
        # If someone reads this comment and understands why this is being done, please
        # update this comment to explain.
        init_devnet_l1_deploy_config(paths, update_timestamp=True)
        run_command([
            'go', 'run', 'cmd/main.go', 'genesis', 'l1',
            '--deploy-config', paths.devnet_config_path,
            '--l1-allocs', paths.allocs_l1_path,
            '--l1-deployments', paths.addresses_json_path,
            '--outfile.l1', paths.genesis_l1_path,
        ], cwd=paths.op_node_dir)

        run_command([
          'sh', 'l1-generate-beacon-genesis.sh',
        ], cwd=paths.ops_bedrock_dir)

    log.info('Starting L1.')
    run_command(['docker', 'compose', 'up', '-d', 'l1', 'l1-bn', 'l1-vc'], cwd=paths.ops_bedrock_dir, env={
        'PWD': paths.ops_bedrock_dir
    })
    wait_up(8545)
    wait_for_rpc_server('127.0.0.1:8545')

    if os.path.exists(paths.genesis_l2_path):
        log.info('L2 genesis and rollup configs already generated.')
    else:
        log.info('Generating L2 genesis and rollup configs.')
        l2_allocs_path = pjoin(paths.devnet_dir, f'allocs-l2-{FORKS[-1]}.json')
        if os.path.exists(l2_allocs_path) == False or DEVNET_L2OO == True:
            # Also regenerate if L2OO.
            # The L2OO flag may affect the L1 deployments addresses, which may affect the L2 genesis.
            devnet_l2_allocs(paths)
        else:
            log.info('Re-using existing L2 allocs.')

        run_command([
            'go', 'run', 'cmd/main.go', 'genesis', 'l2',
            '--l1-rpc', 'http://localhost:8545',
            '--deploy-config', paths.devnet_config_path,
            '--l2-allocs', l2_allocs_path,
            '--l1-deployments', paths.addresses_json_path,
            '--outfile.l2', paths.genesis_l2_path,
            '--outfile.rollup', paths.rollup_config_path
        ], cwd=paths.op_node_dir)

    rollup_config = read_json(paths.rollup_config_path)
    addresses = read_json(paths.addresses_json_path)

    # Start the L2.
    log.info('Bringing up L2.')
    run_command(['docker', 'compose', 'up', '-d', 'l2'], cwd=paths.ops_bedrock_dir, env={
        'PWD': paths.ops_bedrock_dir
    })

    # Wait for the L2 to be available.
    wait_up(9545)
    wait_for_rpc_server('127.0.0.1:9545')

    # Print out the addresses being used for easier debugging.
    batch_inbox_address = rollup_config['batch_inbox_address']
    log.info(f'Using batch inbox {batch_inbox_address}')

    # Set up the base docker environment.
    docker_env = {
        'PWD': paths.ops_bedrock_dir,
        'SEQUENCER_BATCH_INBOX_ADDRESS': batch_inbox_address
    }

    # Selectively set the L2OO_ADDRESS or DGF_ADDRESS if using L2OO.
    # Must be done selectively because op-proposer throws if both are set.
    if DEVNET_L2OO:
        l2_output_oracle = addresses['L2OutputOracleProxy']
        docker_env['L2OO_ADDRESS'] = l2_output_oracle
        log.info(f'Using L2OutputOracle {l2_output_oracle}')
    else:
        dispute_game_factory = addresses['DisputeGameFactoryProxy']
        log.info(f'Using DisputeGameFactory {dispute_game_factory}')
        docker_env['DGF_ADDRESS'] = dispute_game_factory
        docker_env['DG_TYPE'] = '254'
        docker_env['PROPOSAL_INTERVAL'] = '12s'

    if DEVNET_ALTDA:
        docker_env['ALTDA_ENABLED'] = 'true'
        docker_env['DA_TYPE'] = 'calldata'
    else:
        docker_env['ALTDA_ENABLED'] = 'false'
        docker_env['DA_TYPE'] = 'blobs'

    if GENERIC_ALTDA:
        docker_env['ALTDA_GENERIC_DA'] = 'true'
        docker_env['ALTDA_SERVICE'] = 'true'
    else:
        docker_env['ALTDA_GENERIC_DA'] = 'false'
        docker_env['ALTDA_SERVICE'] = 'false'

    # Bring up the rest of the services.
    log.info('Bringing up `op-node`, `op-proposer` and `op-batcher`.')
    run_command(['docker', 'compose', 'up', '-d', 'op-node', 'op-proposer', 'op-batcher', 'artifact-server'], cwd=paths.ops_bedrock_dir, env=docker_env)

    # Optionally bring up op-challenger.
    if not DEVNET_L2OO:
        log.info('Bringing up `op-challenger`.')
        run_command(['docker', 'compose', 'up', '-d', 'op-challenger'], cwd=paths.ops_bedrock_dir, env=docker_env)

    # Optionally bring up Alt-DA Mode components.
    if DEVNET_ALTDA:
        log.info('Bringing up `da-server`, `sentinel`.') # TODO(10141): We don't have public sentinel images yet
        run_command(['docker', 'compose', 'up', '-d', 'da-server'], cwd=paths.ops_bedrock_dir, env=docker_env)

    # Fin.
    log.info('Devnet ready.')

def wait_for_rpc_server(url):
    log.info(f'Waiting for RPC server at {url}')

    headers = {'Content-type': 'application/json'}
    body = '{"id":1, "jsonrpc":"2.0", "method": "eth_chainId", "params":[]}'

    while True:
        try:
            conn = http.client.HTTPConnection(url)
            conn.request('POST', '/', body, headers)
            response = conn.getresponse()
            if response.status < 300:
                log.info(f'RPC server at {url} ready')
                return
        except Exception as e:
            log.info(f'Waiting for RPC server at {url}')
            time.sleep(1)
        finally:
            if conn:
                conn.close()


CommandPreset = namedtuple('Command', ['name', 'args', 'cwd', 'timeout'])


def run_commands(commands: list[CommandPreset], max_workers=2):
    with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
        futures = [executor.submit(run_command_preset, cmd) for cmd in commands]

        for future in concurrent.futures.as_completed(futures):
            result = future.result()
            if result:
                print(result.stdout)


def run_command_preset(command: CommandPreset):
    with subprocess.Popen(command.args, cwd=command.cwd,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True) as proc:
        try:
            # Live output processing
            for line in proc.stdout:
                # Annotate and print the line with timestamp and command name
                timestamp = datetime.datetime.utcnow().strftime('%H:%M:%S.%f')
                # Annotate and print the line with the timestamp
                print(f"[{timestamp}][{command.name}] {line}", end='')

            stdout, stderr = proc.communicate(timeout=command.timeout)

            if proc.returncode != 0:
                raise RuntimeError(f"Command '{' '.join(command.args)}' failed with return code {proc.returncode}: {stderr}")

        except subprocess.TimeoutExpired:
            raise RuntimeError(f"Command '{' '.join(command.args)}' timed out!")

        except Exception as e:
            raise RuntimeError(f"Error executing '{' '.join(command.args)}': {e}")

        finally:
            # Ensure process is terminated
            proc.kill()
    return proc.returncode


def run_command(args, check=True, shell=False, cwd=None, env=None, timeout=None):
    env = env if env else {}
    return subprocess.run(
        args,
        check=check,
        shell=shell,
        env={
            **os.environ,
            **env
        },
        cwd=cwd,
        timeout=timeout
    )


def wait_up(port, retries=10, wait_secs=1):
    for i in range(0, retries):
        log.info(f'Trying 127.0.0.1:{port}')
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        try:
            s.connect(('127.0.0.1', int(port)))
            s.shutdown(2)
            log.info(f'Connected 127.0.0.1:{port}')
            return True
        except Exception:
            time.sleep(wait_secs)

    raise Exception(f'Timed out waiting for port {port}.')


def write_json(path, data):
    with open(path, 'w+') as f:
        json.dump(data, f, indent='  ')


def read_json(path):
    with open(path, 'r') as f:
        return json.load(f)
