FROM eyes852/ubuntu-iperf-test:0.5

COPY ./bin/loxi-cloud-controller-manager /bin/loxi-cloud-controller-manager
USER root
RUN chmod +x /bin/loxi-cloud-controller-manager