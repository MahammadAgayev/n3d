FROM multani/nomad:1.6.3

# Propogating docker to multani/nomad docker image 

RUN apt-get update

#Add dependencies 
RUN  apt-get -y install iptables ca-certificates curl gnupg iproute2
RUN  install -m 0755 -d /etc/apt/keyrings

# Add Docker's official GPG key:
RUN  curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
RUN  chmod a+r /etc/apt/keyrings/docker.gpg

# Add the repository to Apt sources:
RUN echo \
  "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \
  "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
   tee /etc/apt/sources.list.d/docker.list > /dev/null
RUN apt-get update

RUN apt-get -y install docker-ce docker-ce-cli containerd.io

RUN curl -L -o cni-plugins.tgz "https://github.com/containernetworking/plugins/releases/download/v1.0.0/cni-plugins-linux-$( [ $(uname -m) = aarch64 ] && echo arm64 || echo amd64)"-v1.0.0.tgz && \
  mkdir -p /opt/cni/bin && \
  tar -C /opt/cni/bin -xzf cni-plugins.tgz

RUN apt-get install -y unzip

ENV CONSUL_VERSION=1.17.0
# Download and install Consul binary
RUN curl -o /tmp/consul.zip https://releases.hashicorp.com/consul/${CONSUL_VERSION}/consul_${CONSUL_VERSION}_linux_amd64.zip && \
    unzip /tmp/consul.zip -d /usr/local/bin && \
    rm /tmp/consul.zip

RUN apt remove -y unzip curl

COPY "entrypoint.sh" "/usr/local/bin/entrypoint.sh"
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]