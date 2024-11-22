set shell := ["bash", "-c"]

PARALLEL := num_cpus()

MAP_JUST := "/usr/bin/env -S parallel --shebang --jobs " + PARALLEL + " --colsep ' ' -r " + just_executable()
