set shell := ["bash", "-c"]

PARALLEL_JOBS := num_cpus()

# TODO: this fails in CI for some reason
MAP_JUST := "/usr/bin/env -S parallel --shebang --jobs " + PARALLEL_JOBS + " --colsep ' ' -r " + just_executable()
