pipeline {
  agent {
    node {
      label 'go'
    }

  }
  stages {
    stage('clone code') {
      agent none
      steps {
        container('go') {
          git(credentialsId: 'ecos-k8s-ssh-key', url: 'git@gitlab.act.buaa.edu.cn:ecc/ecos.git', changelog: true, poll: false, branch: 'main')
        }

      }
    }

    stage('unit test') {
      agent none
      steps {
        sh 'echo "test"'
      }
    }

    stage('build & push') {
      agent none
      steps {
        container('go') {
          sh 'docker build -f Dockerfile -t $REGISTRY/$DOCKERHUB_NAMESPACE/$APP_NAME:SNAPSHOT-$BRANCH_NAME-$BUILD_NUMBER .'
          withCredentials([usernamePassword(credentialsId : 'harbor-id' ,usernameVariable : 'DOCKER_USERNAME' ,passwordVariable : 'DOCKER_PASSWORD' ,)]) {
            sh 'docker login $REGISTRY -u $DOCKER_USERNAME -p $DOCKER_PASSWORD'
            sh 'docker push  $REGISTRY/$DOCKERHUB_NAMESPACE/$APP_NAME:SNAPSHOT-$BRANCH_NAME-$BUILD_NUMBER'
          }
        }

      }
    }

    stage('push latest') {
      agent none
      steps {
        container('go') {
          sh 'docker tag  $REGISTRY/$DOCKERHUB_NAMESPACE/$APP_NAME:SNAPSHOT-$BRANCH_NAME-$BUILD_NUMBER $REGISTRY/$DOCKERHUB_NAMESPACE/$APP_NAME:latest '
          sh 'docker push  $REGISTRY/$DOCKERHUB_NAMESPACE/$APP_NAME:latest '
        }

      }
    }

    stage('deploy to dev') {
      agent none
      steps {
        input(id: 'deploy-to-dev', message: 'deploy to dev?')
        kubernetesDeploy(configs: 'deploy/dev-ol/**', enableConfigSubstitution: true, kubeconfigId: 'sincerexia-kubeconfig')
      }
    }

    stage('deploy to production') {
      agent none
      steps {
        input(id: 'deploy-to-production', message: 'deploy to production?')
        kubernetesDeploy(configs: 'deploy/prod-ol/**', enableConfigSubstitution: true, kubeconfigId: 'sincerexia-kubeconfig')
      }
    }

  }
  environment {
    DOCKER_CREDENTIAL_ID = 'harbor-id'
    GITHUB_CREDENTIAL_ID = 'github-id'
    KUBECONFIG_CREDENTIAL_ID = 'sincerexia-kubeconfig'
    REGISTRY = 'harbor.sums.top'
    DOCKERHUB_NAMESPACE = 'ecos'
    GITHUB_ACCOUNT = 'kubesphere'
    APP_NAME = 'ecos-edge-node'
  }
  parameters {
    string(name: 'TAG_NAME', defaultValue: '', description: '')
  }
}