import os
from contextlib import contextmanager


constants = {
        "api_hosts_by_env": {
            "cde": "https://app-control.cde.arryved.com",
            "dev": "https://app-control.dev.arryved.com:1026",
            "prod": "https://app-control.prod.arryved.com:1026",
            "sandbox": "https://app-control.dev.arryved.com:1026",
            "stg": "https://app-control.stg.arryved.com:1026",
        },

        "repo_name": "arryved-app-control-config",
}


@contextmanager
def working_directory(target):
    previous = os.getcwd()
    os.chdir(target)
    try:
        yield "."
    finally:
        os.chdir(previous)
