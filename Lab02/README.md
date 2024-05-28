To run distributed we need to specify environment variables.

For the Coordinator
COORDINATOR_PORT which is the port where the coordinator will listen to the incoming connection from worker

For the workers
COORDINATOR_IP  with format `ip:port` which is how workers can connect to the coordinator.

