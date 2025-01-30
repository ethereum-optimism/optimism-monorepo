"""
This is the main script for the kurtosis test runner.
"""

def run(plan, devnet, timestamp, tests):

    tests_artifact = plan.upload_files(
        src = "./",
        name = "tests",
        description = "uploading tests"
    )

    for test in tests:
        plan.run_sh(
            run = "/bin/bash /tests/{} --devnet {} --environment /tests/{}.json".format(test, devnet, devnet),
            name = "{}-{}".format(test, timestamp),
            image = "mslipper/deployment-utils:latest",
            wait="180s",

            files = {
                "/tests": tests_artifact,
            },
        )
