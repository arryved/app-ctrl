from setuptools import setup, find_packages

setup(
    name='app-control',
    version='1.0.0',

	description='app-control CLI',
    long_description_content_type='text/markdown',
    long_description='''app-control''',

    packages=find_packages(),
    include_package_data=True,

    install_requires=[
        'Click',
        'ansitable>=0.9',
        'click-spinner>=0.1',
        'colorama>=0.4',
        'pyyaml>=6',
    ],

    entry_points={
        'console_scripts': [
            'app-control = appcontrol.cli:cli',
        ],
    },
)
