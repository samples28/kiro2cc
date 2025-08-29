pipeline {
    agent any

    environment {
        GO_VERSION = '1.23.3'
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Setup Go') {
            steps {
                script {
                    def goExists = fileExists(tool: 'Go', name: "go${GO_VERSION}")
                    if (!goExists) {
                        sh "wget https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz"
                        sh "tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz"
                    }
                    env.PATH = "/usr/local/go/bin:${env.PATH}"
                }
            }
        }

        stage('Build') {
            steps {
                sh 'go build -v ./...'
            }
        }

        stage('Test') {
            steps {
                sh 'go test -v ./...'
            }
        }
    }

    post {
        always {
            cleanWs()
        }
    }
}
