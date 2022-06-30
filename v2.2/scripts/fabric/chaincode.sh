#!/usr/bin/env bash
# This script installs and commits a chaincode on an already setup channel of a Fabric network,
# accoridng to Fabric v2 semantics.
# Usage (smallbank is installed and committed on peer0.org1 and peer0.org2):
#   ./chaincode.sh -i -c "Org1MSP:peer0.org1.example.com Org2MSP:peer0.org2.example.com" \
#   smallbank 1.0 github.com/smallbank/go/v2/shim "OR('Org1MSP.member', 'Org2MSP.member')"
set -e

# Arguments
INSTALL=false
COMMIT=false

POSITIONAL=()
while [[ $# -gt 0 ]]; do
key="$1"
case $key in
    # Install chaincode on peers and approve for the org.
    -i)
    INSTALL=true
    shift # past argument
    ;;
    # Commit chaincode.
    -c)
    COMMIT=true
    shift # past argument
    ;;
    *)    # unknown option
    POSITIONAL+=("$1") # save it in an array for later
    shift # past argument
    ;;
esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

CHANNEL=$1
TARGET_PEERS=$2  # Format is "msp:peer" e.g. Org1MSP:peer0.org1.example.com
CC_NAME=$3  # smallbank
CC_VERSION=$4  # 1.0
CC_PATH=$5  # github.com/smallbank/go
CC_POLICY=$6  # "OR('Org1MSP.peer', 'Org2MSP.peer')"

printf "\n=== Packaging chaincode $CC_NAME ===\n\n"
docker exec cli.example.com peer lifecycle chaincode package $CC_NAME.tar.gz --path ${CC_PATH} --label ${CC_NAME}_${CC_VERSION} 1>packagecc.log 2>&1
IFS='.' read -r a b <<< "$CC_VERSION"
package_seq=$(($a + $b))

if [ $INSTALL = true ]; then
    for tp in $TARGET_PEERS; do
        IFS=':' read -r msp peer <<< "$tp"
        IFS='.' read -r tmp org <<< "$peer"

        printf "\n=== Instantiating chaincode $CC_NAME on $CHANNEL : $peer ===\n\n"

        echo "Installing chaincode ..."
        docker exec -e CORE_PEER_ID=$peer -e CORE_PEER_ADDRESS=$peer:7051 -e CORE_PEER_LOCALMSPID=$msp -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/crypto-config/peerOrganizations/$org/users/Admin@$org/msp cli.example.com peer lifecycle chaincode install $CC_NAME.tar.gz 1>installcc.log 2>&1
        #cat installcc.log
        sleep 2s

        echo "Approving chaincode ..."
        package_id=$(cat installcc.log | grep "Chaincode code package identifier:" | awk '{print $13}')
        docker exec -e CORE_PEER_ID=$peer -e CORE_PEER_ADDRESS=$peer:7051 -e CORE_PEER_LOCALMSPID=$msp -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/crypto-config/peerOrganizations/$org/users/Admin@$org/msp cli.example.com peer lifecycle chaincode approveformyorg -o orderer0.example.com:7050 --channelID $CHANNEL --name $CC_NAME --version $CC_VERSION --package-id $package_id --sequence $package_seq --signature-policy "$CC_POLICY" 1>approvecc.log 2>&1
    done
fi

sleep 2s

if [ $COMMIT = true ]; then
    printf "\n=== Committing chaincode $CC_NAME on $CHANNEL ===\n\n"

    # Let's use the first target peer for the cli docker.
    IFS=' ' read -r tp1 tmp <<< "$TARGET_PEERS"
    IFS=':' read -r msp peer <<< "$tp1"
    IFS='.' read -r tmp org <<< "$peer"

    # Peers to endorse committing of the chiancode.
    peer_addresses=""
    for tp in $TARGET_PEERS; do
        IFS=':' read -r tmp p <<< "$tp"
        peer_addresses+="--peerAddresses $p:7051 "
    done

    echo "Checking chaincode commit readiness ..."
    docker exec -e CORE_PEER_ID=$peer -e CORE_PEER_ADDRESS=$peer:7051 -e CORE_PEER_LOCALMSPID=$msp -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/crypto-config/peerOrganizations/$org/users/Admin@$org/msp cli.example.com peer lifecycle chaincode checkcommitreadiness -o orderer0.example.com:7050 --channelID $CHANNEL --name $CC_NAME --version $CC_VERSION --sequence $package_seq --signature-policy "$CC_POLICY" 1>commitcc.log 2>&1

    echo "Committing chaincode ..."
    docker exec -e CORE_PEER_ID=$peer -e CORE_PEER_ADDRESS=$peer:7051 -e CORE_PEER_LOCALMSPID=$msp -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/crypto-config/peerOrganizations/$org/users/Admin@$org/msp cli.example.com peer lifecycle chaincode commit -o orderer0.example.com:7050 --channelID $CHANNEL --name $CC_NAME --version $CC_VERSION --sequence $package_seq --signature-policy "$CC_POLICY" $peer_addresses 1>commitcc.log 2>&1
fi
