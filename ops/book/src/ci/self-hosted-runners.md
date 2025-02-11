# Self Hosted Runners

We use self-hosted runners to run some of our CI jobs. Self-hosted runners let us run jobs on bigger boxes for less
money, and allow us to share build caches and the like between job runs on the same machine. Jobs using self-hosted
runners will use `machine` executors and one of the following resource types:

- `latitude-1`: Generic runner. Used for smaller jobs and Go builds. Has up to 16x concurrency.
- `latitude-1-go-e2e`: Go E2E test runner. Has up to 2x concurrency to avoid overloading the machine, since the e2e
  tests are parallelized across all the machine's cores.
- `latitdue-fps-1`: Dedicated fault proofs builder. Only allows 1 job to run at a time due to how heavy the fault
  proof tests are.

Generally, you should use the default CircleCI Docker runners for new jobs unless you're making heavy use of
Go tools.

## Forking Monorepo CI

It's possible to fork the monorepo and have functioning CI on your own CircleCI account, but it requires some extra 
steps.

OP Labs maintains a set of Ansible playbooks that provision the self-hosted runners. These playbooks are 
closed-source since they contain secrets. However, as a result of using [mise](./mise.md), there is nothing special 
installed on the self-hosted runners that isn't already available in CircleCI's base Docker images. For example, 
here's the Dockerfile we use to configure the runners:

```dockerfile
FROM circleci/runner-agent:machine-3.0.25-6554-32567e6

ARG GO_VERSION=1.22.8

USER root

RUN apt-get update && \
    apt-get install -y \
    curl \
    vim \
    git \
    build-essential \
    clang \
    jq \
    lld \
    binutils \
    ca-certificates \
    parallel

COPY wrapper.sh /var/opt/circleci/wrapper.sh
RUN chmod +x /var/opt/circleci/wrapper.sh

USER circleci:circleci

# Everything below this line is installed locally as the CircleCI user. The CCI user does not have sudo for security
# reasons, so don't put anything here that needs root.

WORKDIR /home/circleci

ENV PATH="$PATH:/usr/local/go/bin:/home/circleci/.foundry/bin:/home/circleci/go/bin:/home/circleci/.cargo/bin:/home/circleci/.local/bin"
ENV CIRCLECI_RUNNER_COMMAND_PREFIX="['/var/opt/circleci/wrapper.sh']"
```

`wrapper.sh` is equally simple:

```bash
#!/bin/bash

set -e

task_agent_cmd=${@:1}

echo "Running CircleCI task agent with command: ${task_agent_cmd}"
# Set up PATH
export PATH="$PATH:/usr/local/go/bin:/home/circleci/.foundry/bin:/home/circleci/go/bin:/home/circleci/.cargo/bin"
# Run the command
$task_agent_cmd
# Collect exit code
exit=$?
echo "CircleCI task agent finished."
exit $exit
```

Using the files above, you can either provision your own self-hosted runners in your monorepo fork or replace usages 
of the self-hosted runners in config.yaml with CircleCI's Docker runners.