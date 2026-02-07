# Oracle Cloud Free ARM Instance Creator (Dockerized)

This project automates the creation of Always Free ARM instances (4 CPUs, 24GB RAM) on Oracle Cloud Infrastructure (OCI).

Due to high demand, creating ARM instances often results in an Out of host capacity error. This script runs within a Docker container, attempting to create the instance every 60 seconds until a slot becomes available.
Key Features of This Fork

    Fully Dockerized: No need to install Python, OCI CLI, or any dependencies on your local machine.

    Automatic Windows CRLF Fix: The container automatically detects and fixes line-ending issues (CRLF) that commonly occur when editing configuration files on Windows, preventing "command not found" errors.

    Organized Structure: All keys and configuration files are isolated within the oci_keys directory.

Prerequisites

    An active account on Oracle Cloud.

    Docker and Docker Compose installed on your machine.

Setup Instructions
1. Prepare Credentials (oci_keys directory)

All credentials must be placed inside the oci_keys directory located in the project root.

Step A: Oracle API Key

    Log in to the Oracle Cloud Console.

    Navigate to My Profile -> API Keys -> Add API Key.

    Select Generate API Key Pair and download the Private Key.

    Save this file as oracle_api_key.pem inside the oci_keys folder.

    Click Add.

    Copy the content displayed in the "Configuration File Preview" text box.

Step B: Config File

    Create a file named config (with no file extension) inside the oci_keys folder.

    Paste the content you copied from the Oracle Console into this file.

    Important: You must edit the key_file path in this file to point to the internal container path. Change the line to look exactly like this:
    Ini, TOML

    key_file=/root/.oci/oracle_api_key.pem

Step C: SSH Keys (For VPS Access)

    Generate an SSH key pair inside the oci_keys folder. This key will be used to access your instance after it is created.
    Bash

    # Run this in your terminal
    ssh-keygen -t rsa -b 4096 -f ./oci_keys/chave_vps_arm

    Do not add a passphrase if you want the process to be fully automated without prompts (optional).

    This will generate two files: chave_vps_arm (Private) and chave_vps_arm.pub (Public).

2. Configure Environment Variables (.env)

Edit the .env file in the project root with your specific Oracle Cloud IDs (OCIDs).

    TENANCY_ID: Your Tenancy OCID.

    AVAILABILITY_DOMAIN: The availability domain ID (e.g., Uocm:SA-VINHEDO-1-AD-1).

    SUBNET_ID: The OCID of the Public Subnet where the instance should be created.

    IMAGE_ID: The OCID of the ARM-compatible image (Ubuntu or Oracle Linux).

    PATH_TO_PUBLIC_SSH_KEY: The internal path to your public key. Keep this as /root/.oci/chave_vps_arm.pub.

Example .env file:
Bash

TENANCY_ID="ocid1.tenancy.oc1..aaaaaaa..."
IMAGE_ID="ocid1.image.oc1.sa-vinhedo-1..."
SUBNET_ID="ocid1.subnet.oc1.sa-vinhedo-1..."
AVAILABILITY_DOMAIN="Uocm:SA-VINHEDO-1-AD-1"
PATH_TO_PUBLIC_SSH_KEY="/root/.oci/chave_vps_arm.pub"

# Instance Resources (Free Tier Limits)
cpus=4
ram=24
bootVolume=100

Usage
Starting the Script

Open your terminal in the project folder and run:
Bash

docker compose up -d --build

Checking Progress

To view the logs and verify the script is running:
Bash

docker compose logs -f

What to expect in the logs:

    ServiceError (500) / Out of host capacity: This is the expected behavior. It means the script is working correctly, but the region is currently full. It will retry automatically every 60 seconds.

    Instance created: This indicates success.

Stopping the Script

Once the instance is created (check your Oracle Cloud Console or email), or if you wish to stop the process:
Bash

docker compose down

File Structure
Plaintext

.
├── docker-compose.yml       # Docker orchestration
├── Dockerfile               # Image definition with CRLF fix
├── .env                     # User configuration variables
├── oracle_cloud_instance_creator.sh # Main script
└── oci_keys/                # Mounted volume for credentials
    ├── config               # OCI Config (edited to point to /root/.oci/)
    ├── oracle_api_key.pem   # API Private Key
    ├── chave_vps_arm        # SSH Private Key
    └── chave_vps_arm.pub    # SSH Public Key

Credits

Based on the original work by futchas.