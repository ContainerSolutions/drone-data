DATA_DIR=/home/adrian/prog/drone-example-data

docker build -t containersol/drone-data .
docker stop drone-data
docker rm drone-data
docker run --name drone-data -p 8081:8081 -d -v $DATA_DIR:/data containersol/drone-data --dev
