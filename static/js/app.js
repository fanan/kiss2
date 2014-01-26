var app = angular.module("app", ["ui.bootstrap"]);

app.factory("Tasks", ['$http', '$log', function($http, $log){
  var tasks = {};
  var url = "/api/tasks/";
  tasks.all = function() {
    $http.get(url)
      .success(function(data){
        $log.debug(data);
        return data;
      })
      .error(function(data){
        $log.error("http request error" + data);
        return data;
      });
  };

  tasks.delete = function(id) {
    $http.delete(url + id);
  };

  tasks.cancel = function(id) {
    $http.put(url + id);
  };

  tasks.start = function(id) {
    $http.post(url + id);
  };

  tasks.archive = function() {
    $http.delete(url);
  };

  return tasks;
}]);

app.factory("PushService", ['$log', '$location', function($log, $location){
  var service = {};
  $log.debug($location.host, $location.port);
  service.connect = function() {
    if (service.ws) {
      return;
    }
    var ws = new WebSocket("ws://" + $location.host() + ":" + $location.port() + "/api/push");
    ws.onopen = function() {
      $log.info("connected");
    };
    ws.onerror = function() {
      $log.error("connection error");
    };
    ws.onmessage = function(message) {
      service.callback(message.data);
    };
    service.ws = ws;
  };
  service.sub = function(cb) {
    service.callback = cb;
  };
  return service;
}]);


app.controller("main", ["$http", "$log", "$scope", "PushService", "Tasks", function($http, $log, $scope, PushService, Tasks){
    //define status
    $scope.status_unstarted = 0;
    $scope.status_waiting = 1;
    $scope.status_downloading = 2;
    $scope.status_combining = 3;
    $scope.status_success = 4;
    $scope.status_failure = 5;

    $scope.action_error = 0;
    $scope.action_delete = 1;
    $scope.action_change = 2;

    $scope.tasks = [];
    $scope.alerts = [];

    //tasks helper functions
    $scope.task_delete = function(task_id) {
      Tasks.delete(task_id);
    };
    $scope.task_cancel = function(task_id) {
      Tasks.cancel(task_id);
    };
    $scope.task_start = function(task_id) {
      Tasks.start(task_id);
    };
    $scope.task_archive = function() {
      Tasks.archive();
    };

    //alerts helper functions
    $scope.alerts_clear_all = function() {
      $scope.alerts.splice(0, $scope.alerts.length);
    };
    $scope.alerts_close = function (idx) {
      $scope.alerts.splice(idx, 1);
    };
    
    //sort helper functions
    $scope.sort_by_name = function() {
      if ($scope.orderProp == "name") {
        $scope.reverse = !$scope.reverse;
      } else {
        $scope.orderProp = "name";
      }
    }

    $scope.sort_by_status = function() {
      if ($scope.orderProp == "status") {
        $scope.reverse = !$scope.reverse;
      } else {
        $scope.orderProp = "status";
      }
    }

    $scope.reverse = true;
    $scope.orderProp = "id";

    // init tasks, only once
    $scope.tasks = [];
    $http.get("/api/tasks")
      .success(function(data){
        $scope.tasks = data;
    }).error(function(data){
      $log.error(data);
    });
    $log.info($scope.tasks);

    // init instructions
    $scope.hide_instruction = true;

    // web socket communication
    PushService.sub(function(data){
      var obj = angular.fromJson(data);
      if (obj.action == $scope.action_change) {
        var found = false;
        var task = obj.data;
        for (var i = $scope.tasks.length - 1; i >= 0; i--) {
          if ($scope.tasks[i].id == task.id) {
            found = true;
            $scope.tasks[i] = task;
            if (task.status == $scope.status_success) {
              $scope.alerts.unshift({msg:(task.name || task.url) + "下载成功!", type:"success"});
              $log.info(task.name + " success")
            }
            if (task.status == $scope.status_failure) {
              $scope.alerts.unshift({msg:(task.name || task.url) + "下载失败: " + task.error, type:"danger"});
              $log.error(task.name + " failure:" + task.error)
            }
            break
          }
        }
        if (!found) {
          $scope.tasks.push(task);
          $scope.alerts.unshift({msg:"新增下载: " + (task.name || task.url) , type:"success"});
        }
      } else {
        if (obj.action == $scope.action_error) {
          $log.error(obj.data);
          $scope.alerts.unshift({msg:"错误: " +  obj.data, type:"danger"});
        } else {
          var id = obj.data;
          for (var i = $scope.tasks.length - 1; i >= 0; i--) {
            if ($scope.tasks[i].id == id) {
              $log.info("task ", id, " deleted");
              $scope.tasks.splice(i, 1);
            }
          }
          $scope.alerts.unshift({msg:"任务:" + id + "已归档" , type:"success"});
        }
      }
      $scope.$apply();
    });

    PushService.connect();
}]);

