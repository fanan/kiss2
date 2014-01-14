//var StatusWaiting = 0;
//var StatusDownloading = 1;
//var StatusCombining = 2;
//var StatusSuccess = 4;
//var StatusFailure = 5;
//var StatusUnstarted = 6;

var currentTime = function() {
  return new Date();
};

var TaskModule = angular.module("TaskAdmin", ["ui.bootstrap"]);

TaskModule.controller("TasksListController",
  function($scope, $http, $timeout, $filter) {

    $scope.StatusWaiting = 0;
    $scope.StatusDownloading = 1;
    $scope.StatusCombining = 2;
    $scope.StatusConverting = 3;
    $scope.StatusSuccess = 4;
    $scope.StatusFailure = 5;
    $scope.StatusUnstarted = 6;

    $scope.alerts = [];
    $scope.tasks = [];
    $scope.fps = 1;
    $scope.timeFormat = "MM-dd HH:mm:ss ";
    $scope.hideInstructions = true;
    $scope.reverse = true;
    var firstTime = true;
    var lastError = false;

    $scope.updateTasks = function() {
      $http.get("/api/tasks").success(function(data) {
        if (!firstTime) {
          angular.forEach(data, function(newTask, idx) {
            if (newTask.status == $scope.StatusFailure || newTask.status == $scope.StatusSuccess) {
              for (var i = $scope.tasks.length - 1; i >= 0; i--) {
                if ($scope.tasks[i].id == newTask.id) {
                  if ($scope.tasks[i].status != newTask.status) {
                    if (newTask.status == $scope.StatusSuccess) {
                      this.unshift({msg:$filter('date')(currentTime(),$scope.timeFormat) + newTask.name + "下载成功", type:"success"});
                    } else {
                      this.unshift({msg:$filter('date')(currentTime(),$scope.timeFormat) + newTask.name + "下载失败", type:"danger"});
                    }
                  }
                  break;
                }
              }
            }
          }, $scope.alerts);
        }
        $scope.tasks = data;
        firstTime = false;
        lastError = false;
      }).error(function(data, httpStatus){
        if (!lastError) {
          $alerts.unshift({msg:$filter('date')(currentTime(),$scope.timeFormat) + data, type:"danger"});
          lastError = true;
        }
      });
      if ($scope.fps < 1) {
        mytimeout = $timeout($scope.updateTasks, 1000);
      } else {
        mytimeout = $timeout($scope.updateTasks, $scope.fps * 1000);
      }
    };

    $scope.orderProp = "status";

    $scope.deleteTask = function(task) {
      var url = "/api/tasks/" + task.id;
      $http.delete(url).success(function(){
        for (var i = $scope.tasks.length - 1; i >= 0; i--) {
          if ($scope.tasks[i].id == task.id) {
            $scope.tasks.splice(i, 1);
            break;
          }
        }
      }).error(function(data, httpStatus){
        $scope.alerts.unshift({msg:$filter('date')(currentTime(),$scope.timeFormat) + "删除任务失败", type:"danger"});
      });
    };

    $scope.downloadTask = function(task) {
      var url = "/api/tasks/" + task.id;
      $http.post(url).success(function(){
        task.status = $scope.StatusWaiting;
      }).error(function(data, httpStatus){
        $scope.alerts.unshift({msg:$filter('date')(currentTime(),$scope.timeFormat) + "启动任务失败", type:"danger"});
      });
    };

    $scope.cancelTask = function(task) {
      var url = "/api/tasks/" + task.id;
      $http.put(url).success(function(){
        if (task.status == $scope.StatusWaiting) {
          task.status = $scope.StatusUnstarted;
        } else {
          task.status = $scope.StatusFailure;
        }
      }).error(function(data, httpStatus){
        $scope.alerts.unshift({msg:$filter('date')(currentTime(),$scope.timeFormat) + "取消失败", type:"danger"});
      });
    };

    var mytimeout = $timeout($scope.updateTasks, 0);

    $scope.closeAlert = function (idx) {
      $scope.alerts.splice(idx, 1);
    };

    $scope.removeAllAlerts = function() {
      $scope.alerts.splice(0, $scope.alerts.length);
    };

    $scope.deleteAllSuccessfulTasks = function() {
      $http.delete("/api/tasks").success(function(){
        var tasks = [];
        angular.forEach($scope.tasks, function(task, idx) {
          if (task.status != $scope.StatusSuccess) {
            this.push(task);
          }
        }, tasks);
        $scope.tasks = tasks;
      }).error(function(data) {
        $scope.alerts.unshift({msg:$filter('date')(currentTime(),$scope.timeFormat) + data, type:"danger"});
      });
    };

    $scope.sortByName = function() {
      if ($scope.orderProp == "name") {
        $scope.reverse = !$scope.reverse;
      }
      $scope.orderProp = "name";
    };

    $scope.sortByStatus = function() {
      if ($scope.orderProp == "status") {
        $scope.reverse = !$scope.reverse;
      }
      $scope.orderProp = "status";
    };
});
